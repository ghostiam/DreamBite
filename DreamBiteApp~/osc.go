package main

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/scgolang/osc"

	"DreamBiteApp/oscquery"
)

func (app *App) setupOSCDispatcher(oscDispatcher *oscQueryDispatcher) error {
	sendGrab := func(v bool, hand Hand) error {
		app.log.Debug("Send Grab",
			slog.String("hand", string(hand)),
			slog.Bool("value", v),
		)

		vrchatAddr, ok := app.vrcOSCAddr()
		if !ok {
			return errors.New("VRChat OSC address not set")
		}

		if hand == "" {
			hand = HandRight
		}

		// Send GrabRight in response
		resp := osc.Message{
			Address:   "/input/Grab" + string(hand),
			Arguments: []osc.Argument{osc.Bool(v)},
			Sender:    nil,
		}
		e := app.oscServer.SendTo(vrchatAddr, resp)
		if e != nil {
			return fmt.Errorf("send OSC message: %w", e)
		}

		return nil
	}

	err := oscDispatcher.AddNodeWithHandler(
		&oscquery.Node{
			FullPath: "/avatar/change",
			Access:   oscquery.AccessReadWrite,
			Type:     oscquery.TypeString,
		},
		func(msg osc.Message, node *oscquery.Node) error {
			app.log.Debug("Received /avatar/change", slog.Any("arguments", msg.Arguments))

			if len(msg.Arguments) == 0 {
				return errors.New("expected at least one argument for /avatar/change")
			}

			avatarID, e := msg.Arguments[0].ReadString()
			if e != nil {
				return fmt.Errorf("read first argument as string: %w", e)
			}

			// Update value
			node.Value = []any{avatarID}

			// Reset hands grab.
			_ = sendGrab(false, HandRight)
			_ = sendGrab(false, HandLeft)

			app.resetAvatar()

			e = app.updateAvatarFromConfig(avatarID)
			if e != nil {
				return fmt.Errorf("update avatar: %w", e)
			}

			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("add avatar change node: %w", err)
	}

	err = oscDispatcher.AddNodeWithHandler(
		&oscquery.Node{
			FullPath: "/avatar/parameters/DreamBite/Grab",
			Access:   oscquery.AccessReadWrite,
			Type:     oscquery.TypeBool,
		},
		func(msg osc.Message, node *oscquery.Node) error {
			app.log.Info("Received /avatar/parameters/DreamBite/Grab", slog.Any("arguments", msg.Arguments))
			if len(msg.Arguments) == 0 {
				return errors.New("expected at least one argument for /avatar/parameters/DreamBite/Grab")
			}

			val, e := msg.Arguments[0].ReadBool()
			if e != nil {
				return fmt.Errorf("read first argument as bool: %w", e)
			}

			// Update value
			node.Value = []any{val}

			app.updateAvatar(func(a *Avatar) {
				a.Grabbed = val
			})

			av := app.avatar()
			e = sendGrab(val, av.HandCollider)
			if e != nil {
				return fmt.Errorf("send grab: %w", e)
			}

			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("add DreamBite/Grab node: %w", err)
	}

	err = oscDispatcher.AddNodeWithHandler(
		&oscquery.Node{
			FullPath: "/avatar/parameters/VRMode",
			Access:   oscquery.AccessReadWrite,
			Type:     oscquery.TypeInt,
		},
		func(msg osc.Message, node *oscquery.Node) error {
			app.log.Info("Received /avatar/parameters/VRMode", slog.Any("arguments", msg.Arguments))
			if len(msg.Arguments) == 0 {
				return errors.New("expected at least one argument for /avatar/parameters/VRMode")
			}

			val, e := msg.Arguments[0].ReadInt32()
			if e != nil {
				return fmt.Errorf("read first argument as int32: %w", e)
			}

			// Update value
			node.Value = []any{val}

			app.log.Info("VRMode changed", slog.Int("value", int(val)))
			app.updateAvatar(func(a *Avatar) {
				a.InVR = val == 1
			})

			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("add VRMode node: %w", err)
	}

	return nil
}

type oscQueryMethod func(msg osc.Message, node *oscquery.Node) error

type oscQueryDispatcher struct {
	oscQuery *oscquery.OscQuery
	handlers map[string]oscQueryMethod
	log      *slog.Logger
}

func newOSCQueryDispatcher(log *slog.Logger) *oscQueryDispatcher {
	return &oscQueryDispatcher{
		oscQuery: oscquery.New(),
		handlers: make(map[string]oscQueryMethod),
		log:      log,
	}
}

func (d *oscQueryDispatcher) AddNodeWithHandler(node *oscquery.Node, handler oscQueryMethod) error {
	d.handlers[node.FullPath] = handler
	//nolint:wrapcheck
	return d.oscQuery.AddNode(node)
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
	node, ok := d.oscQuery.GetNode(msg.Address)
	if !ok {
		return nil
	}

	handler, ok := d.handlers[node.FullPath]
	if !ok {
		return fmt.Errorf("no handler registered for OSC path: %s", node.FullPath)
	}

	err := handler(msg, node)
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
