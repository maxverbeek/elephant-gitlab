package main

import (
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/godbus/dbus/v5"
)

const (
	secretsDest = "org.freedesktop.secrets"
	secretsPath = "/org/freedesktop/secrets"
	defaultColl = "/org/freedesktop/secrets/aliases/default"
	schemaAttr  = "xdg:schema"
	noPrompt    = dbus.ObjectPath("/")
)

type secret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

// matchesAttrs reports whether attrs is exactly {service=gitlab, host=host},
// tolerating the xdg:schema attribute that secret-tool adds.
func matchesAttrs(attrs map[string]string, host string) bool {
	rest := maps.Clone(attrs)
	delete(rest, schemaAttr)
	return len(rest) == 2 && rest["service"] == "gitlab" && rest["host"] == host
}

func lookupToken(host string) (string, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return "", fmt.Errorf("session bus: %w", err)
	}
	defer conn.Close()

	svc := conn.Object(secretsDest, secretsPath)

	var sessionPath dbus.ObjectPath
	if err := svc.Call("org.freedesktop.Secret.Service.OpenSession", 0, "plain", dbus.MakeVariant("")).Store(new(dbus.Variant), &sessionPath); err != nil {
		return "", fmt.Errorf("open session: %w", err)
	}
	defer conn.Object(secretsDest, sessionPath).Call("org.freedesktop.Secret.Session.Close", 0)

	var found []dbus.ObjectPath
	query := map[string]string{"service": "gitlab", "host": host}
	if err := conn.Object(secretsDest, defaultColl).Call("org.freedesktop.Secret.Collection.SearchItems", 0, query).Store(&found); err != nil {
		return "", fmt.Errorf("search items: %w", err)
	}

	var items []dbus.ObjectPath
	for _, p := range found {
		var attrs dbus.Variant
		if err := conn.Object(secretsDest, p).Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.Secret.Item", "Attributes").Store(&attrs); err != nil {
			return "", fmt.Errorf("get attributes: %w", err)
		}
		if m, _ := attrs.Value().(map[string]string); matchesAttrs(m, host) {
			items = append(items, p)
		}
	}

	switch len(items) {
	case 0:
		return "", fmt.Errorf("secret service item service=gitlab host=%s is missing", host)
	case 1:
	default:
		return "", fmt.Errorf("%d secret service items match service=gitlab host=%s, expected exactly one", len(items), host)
	}
	item := conn.Object(secretsDest, items[0])

	var locked dbus.Variant
	if err := item.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.Secret.Item", "Locked").Store(&locked); err != nil {
		return "", fmt.Errorf("get locked: %w", err)
	}
	if l, _ := locked.Value().(bool); l {
		if err := unlock(conn, items[0]); err != nil {
			return "", err
		}
	}

	var s secret
	if err := item.Call("org.freedesktop.Secret.Item.GetSecret", 0, sessionPath).Store(&s); err != nil {
		return "", fmt.Errorf("get secret: %w", err)
	}

	token := strings.TrimSpace(string(s.Value))
	if token == "" {
		return "", fmt.Errorf("secret service item service=gitlab host=%s is empty", host)
	}
	return token, nil
}

func unlock(conn *dbus.Conn, item dbus.ObjectPath) error {
	if err := conn.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.Secret.Prompt"), dbus.WithMatchMember("Completed")); err != nil {
		return fmt.Errorf("match signal: %w", err)
	}
	signals := make(chan *dbus.Signal, 8)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)

	var unlocked []dbus.ObjectPath
	var prompt dbus.ObjectPath
	if err := conn.Object(secretsDest, secretsPath).Call("org.freedesktop.Secret.Service.Unlock", 0, []dbus.ObjectPath{item}).Store(&unlocked, &prompt); err != nil {
		return fmt.Errorf("unlock: %w", err)
	}
	if prompt == noPrompt {
		return nil
	}

	if err := conn.Object(secretsDest, prompt).Call("org.freedesktop.Secret.Prompt.Prompt", 0, "").Err; err != nil {
		return fmt.Errorf("unlock prompt: %w", err)
	}
	for sig := range signals {
		if sig.Path != prompt {
			continue
		}
		if dismissed, _ := sig.Body[0].(bool); dismissed {
			return errors.New("unlock prompt dismissed")
		}
		return nil
	}
	return errors.New("unlock prompt: bus closed")
}
