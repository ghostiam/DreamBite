package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/allan-simon/go-singleinstance"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/grandcat/zeroconf"
	"github.com/scgolang/osc"
)

//nolint:tagliatelle
type OSCQueryNode struct {
	FullPath    string                   `json:"FULL_PATH"`
	Contents    map[string]*OSCQueryNode `json:"CONTENTS,omitempty" exhaustruct:"optional"`
	Access      int                      `json:"ACCESS,omitempty" exhaustruct:"optional"`
	Type        string                   `json:"TYPE,omitempty" exhaustruct:"optional"`
	Value       []any                    `json:"VALUE,omitempty" exhaustruct:"optional"`
	Description string                   `json:"DESCRIPTION,omitempty" exhaustruct:"optional"`
	//
	// TODO:
	//  handler func(message osc.Message, node *OSCQueryNode) error
}

//nolint:tagliatelle
type HostInfo struct {
	Name         string          `json:"NAME"`
	OscPort      int             `json:"OSC_PORT"`
	OscIP        string          `json:"OSC_IP"`
	OscTransport string          `json:"OSC_TRANSPORT"`
	Extensions   map[string]bool `json:"EXTENSIONS"`
}

const appName = "DreamBite"

var version = "dev"

// TODO:
//  add GUI with tray icon
//  auto build OSCQueryNode tree
//  /avatar/parameters/VRMode + TrackingType https://creators.vrchat.com/avatars/animator-parameters/

func main() {
	log := slog.New(
		//nolint:exhaustruct
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}),
	)

	err := run(log)
	if err != nil {
		log.Error("Error occurred", slog.String("err", err.Error()))
		log.Info("Press any key to exit...")
		// Wait for user input.
		var buf [1]byte
		_, _ = os.Stdin.Read(buf[:])
		return
	}
}

func run(log *slog.Logger) error {
	lockFile, err := singleinstance.CreateLockFile(filepath.Join(os.TempDir(), appName+".lock"))
	if err != nil {
		return fmt.Errorf("another instance of %s is already running (%w)", appName, err)
	}
	defer func() {
		_ = lockFile.Close()
	}()

	log.Info("Starting", slog.String("version", version))

	// TODO: get vrchat address from OSCQuery.
	vrchatAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:9000")
	if err != nil {
		return fmt.Errorf("resolve VRChat address: %w", err)
	}
	oscClient, err := osc.DialUDP("udp", nil, vrchatAddr)
	if err != nil {
		return fmt.Errorf("dial VRChat OSC: %w", err)
	}
	defer func() {
		_ = oscClient.Close()
	}()

	// Setup OSC.
	oscServer, err := osc.ListenUDP("udp", nil)
	if err != nil {
		return fmt.Errorf("listen OSC server: %w", err)
	}
	defer func() {
		_ = oscServer.Close()
	}()

	oscAddr, _ := oscServer.LocalAddr().(*net.UDPAddr)
	log.Info("OSC Server listening", slog.Int("port", oscAddr.Port))

	// OSCQuery.
	root := &OSCQueryNode{
		FullPath: "/",
		Contents: make(map[string]*OSCQueryNode),
	}
	root.Contents["avatar"] = &OSCQueryNode{
		FullPath: "/avatar",
		Contents: make(map[string]*OSCQueryNode),
	}
	root.Contents["avatar"].Contents["change"] = &OSCQueryNode{
		FullPath: "/avatar/change",
		Type:     string(osc.TypetagString),
		Access:   3, // Read/Write.
		Value:    []any{""},
	}
	root.Contents["avatar"].Contents["parameters"] = &OSCQueryNode{
		FullPath: "/avatar/parameters",
		Contents: make(map[string]*OSCQueryNode),
	}
	root.Contents["avatar"].Contents["parameters"].Contents["DreamBite"] = &OSCQueryNode{
		FullPath: "/avatar/parameters/DreamBite",
		Contents: make(map[string]*OSCQueryNode),
	}
	root.Contents["avatar"].Contents["parameters"].Contents["DreamBite"].Contents["Grab"] = &OSCQueryNode{
		FullPath: "/avatar/parameters/DreamBite/Grab",
		Type:     string(osc.TypetagTrue),
		Access:   3, // Read/Write.
		Value:    []any{false},
	}

	// HTTP Server.
	r := chi.NewRouter()
	r.Use(middleware.RequestLogger(&middleware.DefaultLogFormatter{Logger: &slogPrint{log}, NoColor: true}))

	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.RequestURI, "HOST_INFO") {
			localAddr, _ := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
			hostIP := "127.0.0.1"
			if tcpAddr, ok := localAddr.(*net.TCPAddr); ok {
				hostIP = tcpAddr.IP.String()
			}

			hostInfo := HostInfo{
				Name:         appName,
				OscPort:      oscAddr.Port,
				OscIP:        hostIP,
				OscTransport: "UDP",
				Extensions: map[string]bool{
					"ACCESS": true,
					"VALUE":  true,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(hostInfo)
			if err != nil {
				slog.Error("Encode host info", slog.String("error", err.Error()))
			}
			return
		}

		path := r.URL.Path
		node := findNode(root, path)
		if node == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(node)
		if err != nil {
			slog.Error("Encode OSCQuery node", slog.String("error", err.Error()))
			return
		}
	})

	httpListener, err := net.Listen("tcp", ":0") //nolint:gosec // allow binding to any interface.
	if err != nil {
		return fmt.Errorf("http listener: %w", err)
	}

	httpAddr, _ := httpListener.Addr().(*net.TCPAddr)
	log.Info("OSCQuery HTTP Server listening", slog.Int("port", httpAddr.Port))

	server := &http.Server{
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
	defer func() {
		_ = server.Close()
	}()

	errCh := make(chan error, 1)
	go func() {
		e := server.Serve(httpListener)
		if e != nil {
			errCh <- fmt.Errorf("http server: %w", e)
		}
	}()

	sendGrab := func(v bool, hand Hand) error {
		// Send GrabRight in response
		resp := osc.Message{
			Address:   "/input/Grab" + string(hand),
			Arguments: []osc.Argument{osc.Bool(v)},
			Sender:    nil,
		}
		err = oscClient.Send(resp)
		if err != nil {
			return fmt.Errorf("send OSC message: %w", err)
		}

		return nil
	}

	// OSC Handlers.
	hand := HandRight
	dispatcher := osc.PatternMatching{
		"/avatar/change": osc.Method(func(msg osc.Message) error {
			if len(msg.Arguments) == 0 {
				return fmt.Errorf("expected at least one argument for /avatar/change")
			}

			// Reset hands grab.
			_ = sendGrab(false, HandRight)
			_ = sendGrab(false, HandLeft)

			avatarID, err := msg.Arguments[0].ReadString()
			if err != nil {
				return fmt.Errorf("read first argument as string: %w", err)
			}

			// Update value in OSCQuery tree
			if node := findNode(root, "/avatar/change"); node != nil {
				node.Value = []any{avatarID}
			}

			// ~\AppData\LocalLow\VRChat\VRChat\OSC\{userId}\Avatars\{avatarId}.json
			filePath := filepath.Join(os.Getenv("USERPROFILE"), "AppData", "LocalLow", "VRChat", "VRChat", "OSC", avatarID+".json")
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("open avatar config file: %w", err)
			}
			defer func() {
				_ = file.Close()
			}()

			var cfg AvatarConfig
			err = json.NewDecoder(file).Decode(&cfg)
			if err != nil {
				return fmt.Errorf("decode avatar config file: %w", err)
			}

			for _, parameter := range cfg.Parameters {
				if parameter.Name == "DreamBite/Marker/RightHand" {
					hand = HandRight
				}
				if parameter.Name == "DreamBite/Marker/LeftHand" {
					hand = HandLeft
				}
			}

			return nil
		}),
		"/avatar/parameters/DreamBite/Grab": osc.Method(func(msg osc.Message) error {
			if len(msg.Arguments) > 0 {
				log.Info("Received DreamBite/Grab", slog.Any("arguments", msg.Arguments))

				val, err := msg.Arguments[0].ReadBool()
				if err != nil {
					return fmt.Errorf("read first argument as bool: %w", err)
				}

				// Update value in OSCQuery tree
				if node := findNode(root, "/avatar/parameters/DreamBite/Grab"); node != nil {
					node.Value = []any{val}
				}

				err = sendGrab(val, hand)
				if err != nil {
					return fmt.Errorf("send grab: %w", err)
				}

				return nil
			}
			return nil
		}),
	}
	go func() {
		e := oscServer.Serve(1, dispatcher)
		if e != nil {
			errCh <- fmt.Errorf("osc server: %w", e)
		}
	}()

	hostname, _ := os.Hostname()

	ips := []string{"127.0.0.1"}
	text := []string{"txtvers=1"}

	ifaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("getting interfaces: %w", err)
	}

	_, err = zeroconf.RegisterProxy(
		appName, "_osc._udp", "local.", oscAddr.Port, hostname, ips, text, ifaces,
	)
	if err != nil {
		return fmt.Errorf("mDNS registering OSC service: %w", err)
	}

	_, err = zeroconf.RegisterProxy(
		appName, "_oscjson._tcp", "local.", httpAddr.Port, hostname, ips, text, ifaces,
	)
	if err != nil {
		return fmt.Errorf("mDNS registering OSC JSON service: %w", err)
	}

	log.Info("OSCQuery service registered via mDNS")

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

func findNode(root *OSCQueryNode, path string) *OSCQueryNode {
	if path == "/" || path == "" {
		return root
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	current := root
	for _, part := range parts {
		next, ok := current.Contents[part]
		if !ok {
			return nil
		}
		current = next
	}
	return current
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

type AvatarConfig struct {
	ID         string                    `json:"id"`
	Name       string                    `json:"name"`
	Parameters []*AvatarConfigParameters `json:"parameters"`
}

type AvatarConfigParameters struct {
	Name   string                 `json:"name"`
	Input  *AvatarConfigParameter `json:"input,omitempty"`
	Output *AvatarConfigParameter `json:"output"`
}

type AvatarConfigParameter struct {
	Address string                    `json:"address"`
	Type    AvatarConfigParameterType `json:"type"`
}

type AvatarConfigParameterType string

const (
	IntType   AvatarConfigParameterType = "Int"
	BoolType  AvatarConfigParameterType = "Bool"
	FloatType AvatarConfigParameterType = "Float"
)
