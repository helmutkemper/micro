//go:build helform_simple

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/helmutkemper/micro/v2/internal/action"
	"github.com/helmutkemper/micro/v2/internal/config"
	"github.com/helmutkemper/micro/v2/internal/screen"
	"github.com/micro-editor/tcell/v2"
)

// Estado do formulário
var formActive bool
var formFields []string
var formIdx int
var formData []string

// Abre o formulário
func helFormOpen() {
	formFields = []string{"Nome", "Idade", "Confirmar (s/n)"}
	formData = make([]string, len(formFields))
	formIdx = 0
	formActive = true
}

// Fecha
func helFormClose() {
	formActive = false
}

// Desenho do formulário (usa drawBox/putString/padRightW do submenu)
func helFormDraw() {
	if !formActive {
		return
	}
	w, h := screen.Screen.Size()
	boxW := 44
	boxH := len(formFields) + 6
	x0 := (w - boxW) / 2
	y0 := (h - boxH) / 2
	st := config.DefStyle

	drawBox(x0, y0, boxW, boxH, " Formulário ")

	y := y0 + 2
	for i, label := range formFields {
		val := formData[i]
		line := fmt.Sprintf("%-16s: %s", label, val)
		sel := st
		if i == formIdx {
			sel = sel.Reverse(true)
		}
		putString(x0+2, y, padRightW(line, boxW-4), sel)
		y++
	}
	putString(x0+2, y0+boxH-2, padRightW("↵ próximo  ⎋ cancelar", boxW-4), st)
}

// Captura de teclas (corrigido: checagem de Ctrl via bitmask)
func helFormHandleKey(ev *tcell.EventKey) bool {
	if !formActive {
		return false
	}

	switch ev.Key() {
	case tcell.KeyEsc:
		helFormClose()
		action.InfoBar.Message("form: cancelado")
		return true

	case tcell.KeyEnter, tcell.KeyTab:
		// muda de campo
		if formIdx < len(formFields)-1 {
			formIdx++
		} else {
			// finaliza
			helFormClose()
			name := strings.TrimSpace(formData[0])

			ageStr := strings.TrimSpace(formData[1])
			age, _ := strconv.Atoi(ageStr)

			confirm := strings.ToLower(strings.TrimSpace(formData[2])) == "s"

			action.InfoBar.Message(fmt.Sprintf("Nome=%s  Idade=%d  Confirmar=%v", name, age, confirm))
		}
		return true

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(formData[formIdx]) > 0 {
			formData[formIdx] = formData[formIdx][:len(formData[formIdx])-1]
		}
		return true

	// setinhas para pular de campo
	case tcell.KeyUp:
		if formIdx > 0 {
			formIdx--
		}
		return true
	case tcell.KeyDown:
		if formIdx < len(formFields)-1 {
			formIdx++
		}
		return true
	}

	// Texto normal (ignora combinações com Ctrl)
	r := ev.Rune()
	if r != 0 && (ev.Modifiers()&tcell.ModCtrl) == 0 {
		formData[formIdx] += string(r)
		return true
	}

	return true
}
