//go:build helmenu_go && helmenu_internal

// PT-BR: Registro de ação usando internal/action.Register
// EN: Action registration via internal/action.Register

package main

import (
	action "github.com/helmutkemper/micro/v2/internal/action"
)

func init() {
	// Registra a ação "helmenu:open" apontando para openHelMenu()
	action.Register("helmenu:open", func() {
		openHelMenu()
	})
}
