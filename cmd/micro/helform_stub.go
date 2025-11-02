//go:build !helform_go

package main

import "github.com/micro-editor/tcell/v2"

// Sem a tag helform_go, estes stubs evitam erros de linkagem.

var formActive bool

func helFormDraw() {}

func helFormOpen() {}

func helFormHandleKey(_ *tcell.EventKey) bool { return false }
