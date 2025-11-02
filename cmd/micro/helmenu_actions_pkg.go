//go:build helmenu_go && helmenu_pkg

// PT-BR: Registro de ação usando pkg/actions.Add
// EN: Action registration via pkg/actions.Add

package main

import (
	actions "github.com/helmutkemper/micro/v2/pkg/actions"
)

func init() {
	actions.Add("helmenu:open", func(args ...string) error {
		openHelMenu()
		return nil
	})
}
