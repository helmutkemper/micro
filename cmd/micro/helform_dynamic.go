//go:build helform_dynamic_go

// rodar: go run -tags "helsubmenu_go,helform_dynamic_go" ./cmd/micro
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

// PT-BR: Tipos de campo | EN: Field kinds
type helFieldKind int

const (
	FText helFieldKind = iota
	FNumber
	FCheckbox
	FList
)

// PT-BR: Definição de um campo | EN: Field definition
type helField struct {
	Label string
	Kind  helFieldKind

	// Texto
	Value string

	// Número
	Min, Max int

	// Checkbox
	Bool bool

	// Lista
	Options []string
	Index   int
}

// Estado do formulário dinâmico
var formActive bool
var formIdx int
var form []*helField

func helFormOpen() {
	// PT-BR: Monte aqui os campos que quiser | EN: Build your form here
	form = []*helField{
		{Label: "Nome", Kind: FText, Value: ""},
		{Label: "Idade", Kind: FNumber, Value: "18", Min: 0, Max: 120},
		{Label: "Aceita termos?", Kind: FCheckbox, Bool: false},
		{Label: "Linguagem", Kind: FList, Options: []string{"Go", "JavaScript", "Rust", "C++"}, Index: 0},
	}
	formIdx = 0
	formActive = true
}

func helFormClose() { formActive = false }

// Desenho do formulário (usa suas helpers de desenho)
func helFormDraw() {
	if !formActive {
		return
	}
	w, h := screen.Screen.Size()
	boxW := 52
	boxH := len(form) + 6
	x0 := (w - boxW) / 2
	y0 := (h - boxH) / 2
	st := config.DefStyle

	drawBox(x0, y0, boxW, boxH, " Formulário dinâmico ")

	y := y0 + 2
	for i, f := range form {
		line := fmt.Sprintf("%-18s: %s", f.Label, renderFieldValue(f))
		s := st
		if i == formIdx {
			s = s.Reverse(true)
		}
		putString(x0+2, y, padRightW(line, boxW-4), s)
		y++
	}
	putString(x0+2, y0+boxH-2, padRightW("↑/↓ mover  ←/→ alterar  Enter próximo/confirmar  Esc cancelar", boxW-4), st)
}

// Como exibir o valor do campo
func renderFieldValue(f *helField) string {
	switch f.Kind {
	case FText:
		return f.Value
	case FNumber:
		return f.Value
	case FCheckbox:
		if f.Bool {
			return "[x]"
		}
		return "[ ]"
	case FList:
		if len(f.Options) == 0 {
			return "(vazio)"
		}
		return f.Options[f.Index]
	default:
		return ""
	}
}

// Entrada de teclas
func helFormHandleKey(ev *tcell.EventKey) bool {
	if !formActive {
		return false
	}
	cur := form[formIdx]

	switch ev.Key() {
	case tcell.KeyEsc:
		helFormClose()
		action.InfoBar.Message("form: cancelado")
		return true

	case tcell.KeyUp:
		if formIdx > 0 {
			formIdx--
		}
		return true
	case tcell.KeyDown:
		if formIdx < len(form)-1 {
			formIdx++
		}
		return true

	case tcell.KeyLeft:
		switch cur.Kind {
		case FList:
			if len(cur.Options) > 0 {
				cur.Index = (cur.Index - 1 + len(cur.Options)) % len(cur.Options)
			}
			return true
		case FNumber:
			n := parseIntDefault(cur.Value, 0)
			if n > cur.Min {
				n--
			}
			cur.Value = strconv.Itoa(n)
			return true
		}
	case tcell.KeyRight:
		switch cur.Kind {
		case FList:
			if len(cur.Options) > 0 {
				cur.Index = (cur.Index + 1) % len(cur.Options)
			}
			return true
		case FNumber:
			n := parseIntDefault(cur.Value, 0)
			if n < cur.Max {
				n++
			}
			cur.Value = strconv.Itoa(n)
			return true
		}

	case tcell.KeyEnter, tcell.KeyTab:
		// próximo campo; se for o último, confirma
		if formIdx < len(form)-1 {
			formIdx++
			return true
		}
		// confirmar
		helFormClose()
		name := strings.TrimSpace(form[0].Value)
		age := parseIntDefault(form[1].Value, 18)
		terms := form[2].Bool
		lang := form[3].Options[form[3].Index]
		action.InfoBar.Message(fmt.Sprintf("OK: Nome=%s  Idade=%d  Termos=%v  Linguagem=%s", name, age, terms, lang))
		return true
	}

	// Texto / edição por tipo
	r := ev.Rune()
	if r != 0 && (ev.Modifiers()&tcell.ModCtrl) == 0 {
		switch cur.Kind {
		case FText:
			cur.Value += string(r)
			return true
		case FNumber:
			if r >= '0' && r <= '9' {
				tmp := cur.Value + string(r)
				n := parseIntDefault(tmp, 0)
				if n >= cur.Min && n <= cur.Max {
					cur.Value = tmp
				}
			}
			return true
		case FCheckbox:
			if r == ' ' || r == 'x' || r == 'X' {
				cur.Bool = !cur.Bool
			}
			return true
		case FList:
			// atalhos: g/G = Go, r/R = Rust, etc. (opcional)
			return true
		}
	}

	// Backspace universal p/ texto/número
	if ev.Key() == tcell.KeyBackspace || ev.Key() == tcell.KeyBackspace2 {
		switch cur.Kind {
		case FText, FNumber:
			if len(cur.Value) > 0 {
				cur.Value = cur.Value[:len(cur.Value)-1]
			}
			return true
		}
	}

	return true
}

func parseIntDefault(s string, d int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return d
	}
	return n
}
