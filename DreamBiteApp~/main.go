package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/allan-simon/go-singleinstance"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/grandcat/zeroconf"
	"github.com/scgolang/osc"

	"DreamBiteApp/oscquery"
	"DreamBiteApp/vrc"
)

const (
	appName                = "DreamBite"
	oscServerPort          = 0
	oscQueryHTTPServerPort = 0
)

var defaultVRCOSCAddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9000, Zone: ""}

var version = "dev"

// TODO:
//  add GUI with tray icon

func main() {
	log := slog.New(
		//nolint:exhaustruct
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}),
	)

	app := newApp(log)

	err := app.Run(log)
	if err != nil {
		log.Error("Error occurred", slog.String("err", err.Error()))
		log.Info("Press any key to exit...")
		// Wait for user input.
		var buf [1]byte
		_, _ = os.Stdin.Read(buf[:])
		return
	}
}

type App struct {
	log        *slog.Logger
	dialer     *net.Dialer
	httpClient *http.Client

	mx             sync.RWMutex `exhaustruct:"optional"`
	vrchatOSCAddr  *net.UDPAddr
	vrchatHTTPAddr *net.TCPAddr
	currentAvatar  *Avatar

	oscServer *osc.UDPConn
}

type Avatar struct {
	ID           string
	Name         string
	HandCollider Hand
	Grabbed      bool
	InVR         bool
}

func (a Avatar) copy() *Avatar {
	return &Avatar{
		ID:           a.ID,
		Name:         a.Name,
		HandCollider: a.HandCollider,
		Grabbed:      a.Grabbed,
		InVR:         a.InVR,
	}
}

func newApp(log *slog.Logger) *App {
	return &App{
		log: log,
		dialer: &net.Dialer{
			Timeout: 5 * time.Second,
		},
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		vrchatOSCAddr:  defaultVRCOSCAddr,
		vrchatHTTPAddr: nil,
		currentAvatar:  &Avatar{}, //nolint:exhaustruct
		oscServer:      nil,
	}
}

func (app *App) Run(log *slog.Logger) error {
	lockFile, err := singleinstance.CreateLockFile(filepath.Join(os.TempDir(), appName+".lock"))
	if err != nil {
		return fmt.Errorf("another instance of %s is already running (%w)", appName, err)
	}
	defer func() {
		_ = lockFile.Close()
	}()

	log.Info("Starting", slog.String("version", version))

	errCh := make(chan error, 1)

	go func() {
		e := app.runResolver()
		if e != nil {
			errCh <- fmt.Errorf("resolver: %w", e)
		}
	}()

	// Setup OSC.
	oscServerAddr := &net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: oscServerPort, Zone: ""}
	app.oscServer, err = osc.ListenUDP("udp", oscServerAddr)
	if err != nil {
		return fmt.Errorf("listen OSC server: %w", err)
	}
	defer func() {
		_ = app.oscServer.Close()
	}()

	oscAddr, _ := app.oscServer.LocalAddr().(*net.UDPAddr)
	app.log.Info("OSC Server listening", slog.Int("port", oscAddr.Port))

	oscDispatcher := newOSCQueryDispatcher(log)

	err = app.setupOSCDispatcher(oscDispatcher)
	if err != nil {
		return fmt.Errorf("setup OSC dispatcher: %w", err)
	}

	go func() {
		e := app.oscServer.Serve(1, oscDispatcher)
		if e != nil {
			errCh <- fmt.Errorf("osc server: %w", e)
		}
	}()

	// OSC Query.
	oscDispatcher.oscQuery.SetHostInfo(&oscquery.HostInfo{
		Name:         appName,
		OscIP:        "",
		OscPort:      oscAddr.Port,
		OscTransport: "",
		Extensions:   nil,
	})

	// HTTP Server.
	r := chi.NewRouter()
	r.Use(middleware.RequestLogger(&middleware.DefaultLogFormatter{Logger: &slogPrint{log}, NoColor: true}))
	r.Handle("/*", oscDispatcher.oscQuery)

	httpListener, err := net.Listen("tcp", ":"+strconv.Itoa(oscQueryHTTPServerPort)) //nolint:noctx
	if err != nil {
		return fmt.Errorf("http listener: %w", err)
	}

	httpAddr, _ := httpListener.Addr().(*net.TCPAddr)
	app.log.Info("OSCQuery HTTP Server listening", slog.Int("port", httpAddr.Port))

	server := &http.Server{
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
	defer func() {
		_ = server.Close()
	}()

	go func() {
		e := server.Serve(httpListener)
		if e != nil {
			errCh <- fmt.Errorf("http server: %w", e)
		}
	}()

	// mDNS
	err = app.RegisterMDNS(oscAddr.Port, httpAddr.Port)
	if err != nil {
		return fmt.Errorf("register mDNS: %w", err)
	}

	app.log.Info("OSCQuery service registered via mDNS")

	// Clean exit.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sig:
		log.Info("Shutting down.")
		return nil

	case err = <-errCh:
		return err
	}
}

func (app *App) RegisterMDNS(oscAddrPort, httpAddrPort int) error {
	hostname, _ := os.Hostname()

	ips := []string{"127.0.0.1"}
	text := []string{"txtvers=1"}

	ifaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("interfaces: %w", err)
	}

	_, err = zeroconf.RegisterProxy(
		appName, "_osc._udp", "local.", oscAddrPort, hostname, ips, text, ifaces,
	)
	if err != nil {
		return fmt.Errorf("mDNS registering OSC service: %w", err)
	}

	_, err = zeroconf.RegisterProxy(
		appName, "_oscjson._tcp", "local.", httpAddrPort, hostname, ips, text, ifaces,
	)
	if err != nil {
		return fmt.Errorf("mDNS registering OSC JSON service: %w", err)
	}

	return nil
}

func (app *App) runResolver() error {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return fmt.Errorf("create resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			if !strings.HasPrefix(entry.Instance, "VRChat-Client") {
				continue
			}

			// VRChat allows connecting to OSCQuery only locally.
			addr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: entry.Port, Zone: ""}

			app.log.Debug("Found VRChat service",
				slog.String("instance", entry.Instance),
				slog.String("addr", addr.String()),
			)

			app.setVrcHTTPAddr(addr)

			go func() {
				err = app.updateInfoFromVRCOSCQuery()
				if err != nil {
					app.log.Error("updateInfoFromVRCOSCQuery", slog.String("error", err.Error()))
				}
			}()
		}
		app.log.Warn("No more entries.")
	}(entries)

	err = resolver.Browse(context.Background(), "_oscjson._tcp", "local.", entries)
	if err != nil {
		return fmt.Errorf("browse for OSCQuery: %w", err)
	}

	return nil
}

func (app *App) updateAvatarFromConfig(avatarID string) error {
	avatarConfig, err := vrc.GetAvatarConfig(avatarID)
	if err != nil {
		return fmt.Errorf("get avatar config: %w", err)
	}

	var hand Hand
	for _, parameter := range avatarConfig.Parameters {
		if parameter.Name == "DreamBite/Marker/RightHand" {
			hand = HandRight
		}
		if parameter.Name == "DreamBite/Marker/LeftHand" {
			hand = HandLeft
		}
	}

	app.updateAvatar(func(a *Avatar) {
		a.ID = avatarID
		a.Name = avatarConfig.Name
		a.HandCollider = hand
	})

	return nil
}

func (app *App) updateInfoFromVRCOSCQuery() error {
	ctx, ctxC := context.WithTimeout(context.Background(), 5*time.Second)
	defer ctxC()

	// Get VRChat OSC port.
	{
		hostInfo, err := app.getVRCOSCQueryHostInfo(ctx)
		if err != nil {
			return fmt.Errorf("get VRChat OSC query host info: %w", err)
		}
		app.setVrcOSCAddr(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: hostInfo.OscPort, Zone: ""})
	}

	// Update Avatar.
	{
		node, err := app.getVRCOSCQueryNode(ctx, "/avatar/change")
		if err != nil {
			return fmt.Errorf("get VRChat OSC query node: /avatar/change: %w", err)
		}

		if len(node.Value) == 0 {
			return errors.New("expected at least one value for /avatar/change")
		}

		avatarID, ok := node.Value[0].(string)
		if !ok {
			return errors.New("expected avatar ID to be a string")
		}

		err = app.updateAvatarFromConfig(avatarID)
		if err != nil {
			return fmt.Errorf("update avatar: %w", err)
		}
	}

	return nil
}

func (app *App) getVRCOSCQueryHostInfo(ctx context.Context) (*oscquery.HostInfo, error) {
	vrchatAddr, ok := app.vrcHTTPAddr()
	if !ok {
		return nil, errors.New("VRChat HTTP address not set")
	}

	u := "http://" + vrchatAddr.String() + "/?HOST_INFO"
	resp, err := httpGet(ctx, app.httpClient, u)
	if err != nil {
		return nil, fmt.Errorf("get: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var info oscquery.HostInfo
	err = json.NewDecoder(resp.Body).Decode(&info)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return &info, nil
}

func (app *App) getVRCOSCQueryNode(ctx context.Context, method string) (*oscquery.Node, error) {
	vrchatAddr, ok := app.vrcHTTPAddr()
	if !ok {
		return nil, errors.New("VRChat HTTP address not set")
	}

	u := "http://" + vrchatAddr.String() + method
	resp, err := httpGet(ctx, app.httpClient, u)
	if err != nil {
		return nil, fmt.Errorf("get: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var node oscquery.Node
	err = json.NewDecoder(resp.Body).Decode(&node)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return &node, nil
}

func (app *App) vrcOSCAddr() (*net.UDPAddr, bool) {
	app.mx.RLock()
	addr := app.vrchatOSCAddr
	app.mx.RUnlock()

	return addr, addr != nil
}

func (app *App) setVrcOSCAddr(addr *net.UDPAddr) {
	app.mx.Lock()
	app.vrchatOSCAddr = addr
	app.mx.Unlock()

	app.log.Debug("SetVrcOSCAddr", slog.Any("addr", addr))
}

func (app *App) vrcHTTPAddr() (*net.TCPAddr, bool) {
	app.mx.RLock()
	addr := app.vrchatHTTPAddr
	app.mx.RUnlock()

	return addr, addr != nil
}

func (app *App) setVrcHTTPAddr(addr *net.TCPAddr) {
	app.mx.Lock()
	app.vrchatHTTPAddr = addr
	app.mx.Unlock()

	app.log.Debug("SetVrcHTTPAddr", slog.Any("addr", addr))
}

func (app *App) avatar() *Avatar {
	app.mx.RLock()
	avatar := app.currentAvatar.copy()
	app.mx.RUnlock()

	return avatar
}

func (app *App) updateAvatar(fn func(a *Avatar)) {
	app.mx.Lock()
	defer app.mx.Unlock()

	fn(app.currentAvatar)

	app.log.Debug("Updated avatar", slog.Any("avatar", app.currentAvatar))
}

func (app *App) resetAvatar() {
	app.mx.Lock()
	defer app.mx.Unlock()

	app.currentAvatar = &Avatar{} //nolint:exhaustruct
}

func httpGet(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	return client.Do(req) //nolint:wrapcheck
}

type slogPrint struct {
	log *slog.Logger
}

func (s *slogPrint) Print(v ...any) {
	s.log.Info(fmt.Sprint(v...)) //nolint:sloglint
}

type Hand string

const (
	HandLeft  Hand = "Left"
	HandRight Hand = "Right"
)
