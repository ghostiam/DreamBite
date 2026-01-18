package oscquery

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
)

type OscQuery[Handler any] struct {
	mx       sync.RWMutex `exhaustruct:"optional"`
	tree     *nodeTree[Handler]
	hostInfo *HostInfo
}

func New[Handler any]() (*OscQuery[Handler], error) {
	return &OscQuery[Handler]{
		tree:     newNodeTree[Handler](),
		hostInfo: nil,
	}, nil
}

func (o *OscQuery[Handler]) AddEndpoint(ep *Endpoint[Handler]) error {
	o.mx.RLock()
	defer o.mx.RUnlock()

	return o.tree.add(ep)
}

func (o *OscQuery[Handler]) GetEndpoint(fullPath string) (Node[Handler], bool) {
	o.mx.RLock()
	defer o.mx.RUnlock()
	return o.tree.find(fullPath)
}

func (o *OscQuery[Handler]) SetHostInfo(info *HostInfo) {
	o.mx.Lock()
	defer o.mx.Unlock()

	o.hostInfo = info
}

func (o *OscQuery[Handler]) HostInfo() *HostInfo {
	o.mx.RLock()
	defer o.mx.RUnlock()

	return o.hostInfo
}

func (o *OscQuery[Handler]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.RequestURI, "HOST_INFO") {
		o.handleHostInfo(w, r)
		return
	}

	path := r.URL.Path
	n, ok := o.tree.find(path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(n)
}

func (o *OscQuery[Handler]) handleHostInfo(w http.ResponseWriter, r *http.Request) {
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

	if hostInfo.OscIP != "" {
		hostIP = hostInfo.OscIP
	}
	oscTransport := "UDP"
	if hostInfo.OscTransport == "TCP" {
		oscTransport = "TCP"
	}

	_ = json.NewEncoder(w).Encode(struct {
		Name         string          `json:"NAME,omitempty"`
		OscIP        string          `json:"OSC_IP,omitempty"`
		OscPort      int             `json:"OSC_PORT,omitempty"`
		OscTransport string          `json:"OSC_TRANSPORT,omitempty"`
		Extensions   map[string]bool `json:"EXTENSIONS"`
	}{
		Name:         hostInfo.Name,
		OscPort:      hostInfo.OscPort,
		OscIP:        hostIP,
		OscTransport: oscTransport,
		Extensions: map[string]bool{
			"ACCESS": true,
			"VALUE":  true,
		},
	})
	return
}
