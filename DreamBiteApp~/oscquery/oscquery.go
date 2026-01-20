package oscquery

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
)

type OscQuery struct {
	mx       sync.RWMutex `exhaustruct:"optional"`
	tree     *Node
	hostInfo *HostInfo
}

func New() *OscQuery {
	return &OscQuery{
		tree:     NewNodeTree(),
		hostInfo: nil,
	}
}

func (o *OscQuery) AddNode(n *Node) error {
	o.mx.RLock()
	defer o.mx.RUnlock()

	return o.tree.Add(n)
}

func (o *OscQuery) GetNode(fullPath string) (*Node, bool) {
	o.mx.RLock()
	defer o.mx.RUnlock()

	return o.tree.Find(fullPath)
}

func (o *OscQuery) SetHostInfo(info *HostInfo) {
	o.mx.Lock()
	defer o.mx.Unlock()

	o.hostInfo = info
}

func (o *OscQuery) HostInfo() *HostInfo {
	o.mx.RLock()
	defer o.mx.RUnlock()

	return o.hostInfo
}

func (o *OscQuery) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.RequestURI, "HOST_INFO") {
		o.handleHostInfo(w, r)
		return
	}

	path := r.URL.Path
	n, ok := o.tree.Find(path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(n) //nolint:errchkjson
}

func (o *OscQuery) handleHostInfo(w http.ResponseWriter, r *http.Request) {
	hostInfo := o.HostInfo()
	if hostInfo == nil {
		http.NotFound(w, r)
		return
	}

	hostIP := "127.0.0.1"
	localAddr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if ok {
		tcpAddr, ok := localAddr.(*net.TCPAddr)
		if ok {
			hostIP = tcpAddr.IP.String()
		}
	}

	w.Header().Set("Content-Type", "application/json")

	if hostInfo.OscIP == "" {
		hostInfo.OscIP = hostIP
	}
	if hostInfo.OscTransport != "TCP" {
		hostInfo.OscTransport = "UDP"
	}

	//nolint:errchkjson
	_ = json.NewEncoder(w).Encode(hostInfo)
	return
}
