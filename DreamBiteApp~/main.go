package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/allan-simon/go-singleinstance"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/grandcat/zeroconf"
	"github.com/scgolang/osc"

	"DreamBiteApp/oscquery"
)

const appName = "DreamBite"

var version = "dev"

// TODO:
//  add GUI with tray icon
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
	oscServerAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	if err != nil {
		return fmt.Errorf("resolve OSC server address: %w", err)
	}
	oscServer, err := osc.ListenUDP("udp", oscServerAddr)
	if err != nil {
		return fmt.Errorf("listen OSC server: %w", err)
	}
	defer func() {
		_ = oscServer.Close()
	}()

	oscAddr, _ := oscServer.LocalAddr().(*net.UDPAddr)
	log.Info("OSC Server listening", slog.Int("port", oscAddr.Port))

	// OSC Query.

	oscQuery, err := oscquery.New[oscQueryMethod]()
	if err != nil {
		return fmt.Errorf("create OSCQuery: %w", err)
	}
	oscQuery.SetHostInfo(&oscquery.HostInfo{
		Name:    appName,
		OscPort: oscAddr.Port,
	})

	// HTTP Server.
	r := chi.NewRouter()
	r.Use(middleware.RequestLogger(&middleware.DefaultLogFormatter{Logger: &slogPrint{log}, NoColor: true}))
	r.Handle("/*", oscQuery)

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
		log.Debug("Send Grab", slog.Any("hand", string(hand)), slog.Bool("value", v))

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

	hand := HandRight
	err = oscQuery.AddEndpoint(&oscquery.Endpoint[oscQueryMethod]{
		FullPath: "/avatar/change",
		Access:   oscquery.AccessReadWrite,
		Type:     oscquery.TypeString,
		Handler: func(msg osc.Message, node oscquery.Node[oscQueryMethod]) error {
			log.Debug("Received /avatar/change", slog.Any("arguments", msg.Arguments))

			if len(msg.Arguments) == 0 {
				return errors.New("expected at least one argument for /avatar/change")
			}

			// Reset hands grab.
			_ = sendGrab(false, HandRight)
			_ = sendGrab(false, HandLeft)

			avatarID, err := msg.Arguments[0].ReadString()
			if err != nil {
				return fmt.Errorf("read first argument as string: %w", err)
			}

			// Update value
			node.SetValue([]any{avatarID})

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
		},
	})
	if err != nil {
		return fmt.Errorf("add avatar change endpoint: %w", err)
	}

	err = oscQuery.AddEndpoint(&oscquery.Endpoint[oscQueryMethod]{
		FullPath: "/avatar/parameters/DreamBite/Grab",
		Access:   oscquery.AccessReadWrite,
		Type:     oscquery.TypeString,
		Handler: func(msg osc.Message, node oscquery.Node[oscQueryMethod]) error {
			if len(msg.Arguments) > 0 {
				log.Info("Received /avatar/parameters/DreamBite/Grab", slog.Any("arguments", msg.Arguments))

				val, err := msg.Arguments[0].ReadBool()
				if err != nil {
					return fmt.Errorf("read first argument as bool: %w", err)
				}

				// Update value
				node.SetValue([]any{val})

				err = sendGrab(val, hand)
				if err != nil {
					return fmt.Errorf("send grab: %w", err)
				}

				return nil
			}
			return nil
		},
	})
	if err != nil {
		return fmt.Errorf("add avatar change endpoint: %w", err)
	}

	// OSC Handlers.
	go func() {
		e := oscServer.Serve(1, &oscQueryDispatcher{oscQuery, log})
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

type oscQueryMethod func(msg osc.Message, node oscquery.Node[oscQueryMethod]) error

type oscQueryDispatcher struct {
	oscQuery *oscquery.OscQuery[oscQueryMethod]
	log      *slog.Logger
}

func (d *oscQueryDispatcher) Dispatch(b osc.Bundle, exactMatch bool) error {
	for _, p := range b.Packets {
		err := d.invoke(p, exactMatch)
		if err != nil {
			d.log.Error("Dispatch", slog.String("err", err.Error()))
		}
	}

	return nil
}

func (d *oscQueryDispatcher) Invoke(msg osc.Message, _ bool) error {
	ep, ok := d.oscQuery.GetEndpoint(msg.Address)
	if !ok {
		d.log.Debug("No endpoint found", slog.String("address", msg.Address))
		return nil
	}

	h := ep.Handler()
	if h == nil {
		return fmt.Errorf("endpoint handler not set: %s", msg.Address)
	}

	err := h(msg, ep)
	if err != nil {
		d.log.Error("Invoke", slog.String("err", err.Error()))
	}

	return nil
}

func (d *oscQueryDispatcher) invoke(p osc.Packet, exactMatch bool) error {
	switch x := p.(type) {
	case osc.Message:
		return d.Invoke(x, exactMatch)
	case osc.Bundle:
		return d.Dispatch(x, exactMatch)
	default:
		return fmt.Errorf("unsupported type for dispatcher: %T", p)
	}
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
