//go:build helsubmenu_go

// go run -tags "helsubmenu_go,helform_dynamic_go" ./cmd/micro

package main

import (
	"fmt"
	"strings"
	"time"

	// PT-BR: Imports internos do seu fork
	// EN:    Internal imports from your fork
	"github.com/helmutkemper/micro/v2/internal/action"
	"github.com/helmutkemper/micro/v2/internal/config"
	"github.com/helmutkemper/micro/v2/internal/screen"
	"github.com/mattn/go-runewidth"
	"github.com/micro-editor/tcell/v2"
)

//
// ============================
// Hel Menu with Submenus (Go)
// ============================
//
// PT-BR: Este arquivo implementa um menu modal com submenus usando tcell.
//        - Abre com Alt-M (ou Esc,m no macOS) via micro.go
//        - Navegação: ↑/↓, Enter, teclas rápidas (key), Esc/Backspace para voltar
//        - Usa timerChan para disparar ações no loop principal
//
// EN:    This file implements a modal menu with submenus using tcell.
//        - Open with Alt-M (or Esc,m on macOS) via micro.go
//        - Navigation: ↑/↓, Enter, quick keys (key), Esc/Backspace to go back
//        - Uses timerChan to fire actions on the main loop
//

// ----------------------------
// Types / Tipos
// ----------------------------

type helMenuItem struct {
	key    rune                   // PT-BR: tecla rápida (opcional) | EN: quick key (optional)
	title  string                 // rótulo / label
	action func() (string, error) // PT-BR: ação de execução | EN: action to execute
	sub    *helMenu               // PT-BR: submenu (se não for ação) | EN: submenu (if not action)
}

type helMenu struct {
	title string
	items []helMenuItem
	idx   int // PT-BR: item selecionado | EN: selected item
}

// ----------------------------
// State / Estado
// ----------------------------

var helMenuActive bool
var helMenuStack []*helMenu // PT-BR: pilha de menus | EN: menu stack

// ----------------------------
// Root menu / Menu raiz
// ----------------------------

func newRootMenu() *helMenu {
	// Submenu Inserções (exemplo que você já tinha)
	insertSub := &helMenu{
		title: " Inserções ",
		items: []helMenuItem{
			// << -------------------- menu --------------------->>
			{key: 'H', title: "Hsplit (acima/abaixo)", action: func() (string, error) {
				if err := action.HelRunCmd("hsplit"); err != nil {
					return "", err
				}
				return "Split horizontal criado", nil
			}},
			{key: 'V', title: "Vsplit (lado a lado)", action: func() (string, error) {
				if err := action.HelRunCmd("vsplit"); err != nil {
					return "", err
				}
				return "Split vertical criado", nil
			}},
			{key: 'T', title: "Nova aba (tab)", action: func() (string, error) {
				if err := action.HelRunCmd("tab"); err != nil {
					return "", err
				}
				return "Aba criada", nil
			}},
			{key: 'Y', title: "Aba com README.md", action: func() (string, error) {
				if err := action.HelRunCmd("tab", "README.md"); err != nil {
					return "", err
				}
				return "Aba criada com README.md", nil
			}},

			// << -------------------- menu --------------------->>
			{key: 'a', title: `Inserir "estou vivo!"`, action: func() (string, error) {
				go func() {
					txt := "estou vivo!"
					timerChan <- func() {
						for _, r := range txt {
							ev := tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone, "")
							action.Tabs.HandleEvent(ev)
						}
					}
				}()
				return `Inserido: "estou vivo!"`, nil
			}},
			{key: 't', title: "Inserir timestamp (RFC3339)", action: func() (string, error) {
				go func() {
					now := time.Now().Format(time.RFC3339)
					timerChan <- func() {
						for _, r := range now {
							ev := tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone, "")
							action.Tabs.HandleEvent(ev)
						}
						action.Tabs.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone, ""))
					}
				}()
				return "Timestamp inserido", nil
			}},
		},
	}

	// Submenu Ferramentas
	toolsSub := &helMenu{
		title: " Ferramentas ",
		items: []helMenuItem{
			{key: '1', title: "Echo status: estou vivo!", action: func() (string, error) {
				return "HEL-1: estou vivo! ✔", nil
			}},
			{key: '2', title: "Tarefa custom", action: func() (string, error) {
				return "HEL-2: tarefa custom concluída.", nil
			}},
		},
	}

	// Submenu Formulários (abre o seu formulário dinâmico)
	formsSub := &helMenu{
		title: " Formulários ",
		items: []helMenuItem{
			{key: 'f', title: "Abrir formulário…", action: func() (string, error) {
				helFormOpen()
				return "form: aberto", nil
			}},
		},
	}

	// Menu raiz
	return &helMenu{
		title: " Hel Menu ",
		items: []helMenuItem{
			{key: 'i', title: "Inserções →", sub: insertSub},
			{key: 'f', title: "Ferramentas →", sub: toolsSub},
			{key: 'o', title: "Formulários →", sub: formsSub},
			{key: 'q', title: "Fechar", action: func() (string, error) {
				helMenuCloseAll()
				return "helmenu: fechado", nil
			}},
		},
	}
}

// ----------------------------
// Open / Close
// ----------------------------

func helMenuOpen() {
	helMenuStack = []*helMenu{newRootMenu()}
	helMenuActive = true
}

func helMenuCloseAll() {
	helMenuStack = nil
	helMenuActive = false
}

func helCur() *helMenu {
	if len(helMenuStack) == 0 {
		return nil
	}
	return helMenuStack[len(helMenuStack)-1]
}

func helPush(m *helMenu) {
	helMenuStack = append(helMenuStack, m)
}

func helPop() {
	if len(helMenuStack) > 1 {
		helMenuStack = helMenuStack[:len(helMenuStack)-1]
	} else {
		helMenuCloseAll()
	}
}

// ----------------------------
// Draw overlay / Desenho
// ----------------------------

func helMenuDraw() {
	if !helMenuActive {
		return
	}
	cur := helCur()
	if cur == nil {
		return
	}

	w, h := screen.Screen.Size()
	boxW := maxWidth(cur)
	boxH := len(cur.items) + 5 // bordas + título + rodapé
	x0 := (w - boxW - 2) / 2
	y0 := (h - boxH) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	drawBox(x0, y0, boxW, boxH, " "+cur.title+" ")

	// linhas
	y := y0 + 2
	for i, it := range cur.items {
		line := it.title
		if it.sub != nil && !endsWithArrow(line) {
			line += " →"
		}
		line = fmt.Sprintf("[%c] %s", it.key, line)

		st := config.DefStyle
		if i == cur.idx {
			// highlight
			st = st.Reverse(true)
		}
		putString(x0+1, y, padRightW(line, boxW), st)
		y++
	}
	putString(x0+1, y0+boxH-2, padRightW("↑/↓, Enter, teclas, Esc/Backspace: voltar", boxW), config.DefStyle)
}

func endsWithArrow(s string) bool {
	rs := []rune(s)
	return len(rs) > 0 && rs[len(rs)-1] == '→'
}

func maxWidth(m *helMenu) int {
	w := strW(m.title) + 2 // espaço extra no título
	for _, it := range m.items {
		line := fmt.Sprintf("[%c] %s", it.key, it.title)
		if it.sub != nil && !endsWithArrow(line) {
			line += " →"
		}
		if lw := strW(line); lw > w {
			w = lw
		}
	}
	if w < 30 {
		w = 30
	}
	return w
}

// ----------------------------
// Key handling / Teclas
// ----------------------------

func helMenuHandleKey(ev *tcell.EventKey) bool {
	if !helMenuActive {
		return false
	}
	cur := helCur()
	if cur == nil {
		helMenuCloseAll()
		return true
	}

	switch ev.Key() {
	case tcell.KeyEsc, tcell.KeyBackspace, tcell.KeyBackspace2:
		helPop()
		return true
	case tcell.KeyUp:
		if cur.idx > 0 {
			cur.idx--
		} else {
			cur.idx = len(cur.items) - 1
		}
		return true
	case tcell.KeyDown:
		if cur.idx < len(cur.items)-1 {
			cur.idx++
		} else {
			cur.idx = 0
		}
		return true
	case tcell.KeyEnter:
		return helSelect(cur, cur.items[cur.idx])
	}

	// tecla rápida
	r := ev.Rune()
	if r != 0 {
		for _, it := range cur.items {
			if r == it.key {
				return helSelect(cur, it)
			}
		}
	}

	// qualquer outra tecla cancela este nível
	helPop()
	return true
}

func helSelect(cur *helMenu, it helMenuItem) bool {
	if it.sub != nil {
		it.sub.idx = 0
		helPush(it.sub)
		return true
	}
	if it.action != nil {
		msg, err := it.action()
		helMenuCloseAll()
		if err != nil {
			action.InfoBar.Message("helmenu: erro: " + err.Error())
		} else if msg != "" {
			action.InfoBar.Message(msg)
		}
		return true
	}
	helMenuCloseAll()
	return true
}

// ----------------------------
// tcell drawing helpers
// ----------------------------

func drawBox(x, y, w, h int, title string) {
	// w = largura útil (conteúdo); borda total = w + 2
	hor, ver := '─', '│'
	tl, tr, bl, br := '┌', '┐', '└', '┘'
	st := config.DefStyle

	left := x
	right := x + w + 1
	top := y
	bot := y + h - 1

	// Cantos
	screen.Screen.SetContent(left, top, tl, nil, st)
	screen.Screen.SetContent(right, top, tr, nil, st)
	screen.Screen.SetContent(left, bot, bl, nil, st)
	screen.Screen.SetContent(right, bot, br, nil, st)

	// Horizontais
	for i := left + 1; i <= right-1; i++ {
		screen.Screen.SetContent(i, top, hor, nil, st)
		screen.Screen.SetContent(i, bot, hor, nil, st)
	}
	// Verticais
	for j := top + 1; j <= bot-1; j++ {
		screen.Screen.SetContent(left, j, ver, nil, st)
		screen.Screen.SetContent(right, j, ver, nil, st)
	}
	// Miolo
	for yy := top + 1; yy <= bot-1; yy++ {
		for xx := left + 1; xx <= right-1; xx++ {
			screen.Screen.SetContent(xx, yy, ' ', nil, st)
		}
	}
	// Título (centralizado pela largura visual)
	putString(left+1, top, " "+padCenterW(title, w-1)+" ", st)
}

func strW(s string) int { return runewidth.StringWidth(s) }

func padRightW(s string, w int) string {
	sw := runewidth.StringWidth(s)
	if sw >= w {
		// corta visualmente sem “…” (mantém largura w)
		return runewidth.Truncate(s, w, "")
	}
	return s + strings.Repeat(" ", w-sw)
}

func padCenterW(s string, w int) string {
	sw := runewidth.StringWidth(s)
	if sw >= w {
		return runewidth.Truncate(s, w, "")
	}
	left := (w - sw) / 2
	right := w - sw - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func putString(x, y int, s string, st tcell.Style) {
	cx := x
	for _, r := range s {
		w := runewidth.RuneWidth(r)
		if w == 0 {
			w = 1
		}
		screen.Screen.SetContent(cx, y, r, nil, st)
		cx += w
	}
}
