//go:build helmenu_go

package main

import (
	"fmt"
	"time"

	"github.com/helmutkemper/micro/v2/internal/action"
	"github.com/helmutkemper/micro/v2/internal/config"
	"github.com/helmutkemper/micro/v2/internal/screen"
	"github.com/micro-editor/tcell/v2"
)

// Estado global do modal
var helMenuActive bool

//var helLastEsc time.Time

type helItem struct {
	key   rune
	title string
	run   func() (string, error)
}

var helItems = []helItem{
	{key: '1', title: "HEL-1: Dizer 'estou vivo!'", run: func() (string, error) {
		return "HEL-1: estou vivo! ✔", nil
	}},
	{key: '2', title: "HEL-2: Inserir timestamp", run: func() (string, error) {
		return time.Now().Format(time.RFC3339), nil
	}},
	{key: '3', title: "HEL-3: Tarefa custom", run: func() (string, error) {
		// coloque sua lógica aqui
		return "HEL-3: tarefa custom concluída.", nil
	}},
	//{key: 'a', title: `HEL-A: Inserir "estou vivo!" no cursor`, run: func() (string, error) {
	//	helMenuActive = false // fecha o modal
	//
	//	s := "estou vivo!"
	//	go func() {
	//		time.Sleep(5 * time.Millisecond) // dá tempo do loop voltar
	//		for _, r := range s {
	//			ev := tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone, "")
	//			action.Tabs.HandleEvent(ev) // <<< um único caminho
	//		}
	//		// (se quiser quebra de linha):
	//		// action.Tabs.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone, ""))
	//	}()
	//
	//	return `Inserido: "estou vivo!"`, nil
	//}},

	{key: 'a', title: `HEL-A: Inserir "estou vivo!" no cursor`, run: func() (string, error) {
		helMenuActive = false // fecha o modal

		s := "estou vivo!"
		// Agenda a inserção para o loop principal processar já no próximo ciclo
		go func() {
			// Envia UMA função para o timerChan (não bloqueia o loop atual)
			timerChan <- func() {
				for _, r := range s {
					ev := tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone, "")
					action.Tabs.HandleEvent(ev)
				}
				// (opcional) Enter no final:
				// action.Tabs.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone, ""))
			}
		}()

		return `Inserido: "estou vivo!"`, nil
	}},

	//{key: 'a', title: `HEL-A: Inserir "estou vivo!" no cursor`, run: func() (string, error) {
	//	// fecha o modal antes de injetar eventos
	//	helMenuActive = false
	//
	//	s := "estou vivo!"
	//	go func() {
	//		// pequena pausa para garantir que voltamos ao loop
	//		time.Sleep(5 * time.Millisecond)
	//		for _, r := range s {
	//			if r == '\n' {
	//				screen.Screen.PostEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone, ""))
	//				continue
	//			}
	//			screen.Screen.PostEvent(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone, ""))
	//		}
	//
	//		// (opcional) Enter no final:
	//		// action.Tabs.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone, ""))
	//	}()
	//
	//	return `Inserido: "estou vivo!"`, nil
	//}},

	//{key: 'a', title: `HEL-4: Inserir "estou vivo!" no cursor`, run: func() (string, error) {
	//	// fecha o modal antes de injetar eventos
	//	helMenuActive = false
	//
	//	s := "estou vivo!"
	//	var wg sync.WaitGroup
	//	wg.Add(1)
	//	go func() {
	//		// pequena pausa para garantir que voltamos ao loop
	//		time.Sleep(5 * time.Millisecond)
	//		for _, r := range s {
	//			ev := tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone, "")
	//			action.Tabs.HandleEvent(ev)
	//		}
	//		// (opcional) Enter no final:
	//		// action.Tabs.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone, ""))
	//		wg.Done()
	//	}()
	//	wg.Wait()
	//	return `HEL-4: Inserido: "estou vivo!"`, nil
	//}},
}

// Chame isto quando detectar Alt-M
func helMenuOpen() {
	helMenuActive = true
}

// Desenha o overlay por cima da UI normal
func helMenuDraw() {
	if !helMenuActive {
		return
	}
	w, h := screen.Screen.Size()
	boxW := 46
	boxH := len(helItems) + 5 // moldura + título + rodapé
	x0 := (w - boxW - 2) / 2
	y0 := (h - boxH) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	drawBox(x0, y0, boxW, boxH, " Hel Menu (Alt-M) ")
	// linhas
	y := y0 + 2
	for _, it := range helItems {
		line := fmt.Sprintf("[%c] %s", it.key, it.title)
		putString(x0+1, y, padRight(line, boxW), config.DefStyle)
		y++
	}
	putString(x0+1, y0+boxH-2, padRight("ESC/Outra tecla: cancelar", boxW), config.DefStyle)
}

// Trata teclas enquanto o modal está ativo. Retorna true se consumiu.
func helMenuHandleKey(ev *tcell.EventKey) bool {
	if !helMenuActive {
		return false
	}

	if ev.Key() == tcell.KeyEsc {
		helMenuActive = false
		action.InfoBar.Message("helmenu: cancelado")
		return true // consumiu
	}

	r := ev.Rune()
	for _, it := range helItems {
		if r == it.key {
			msg, err := it.run()
			helMenuActive = false
			if err != nil {
				action.InfoBar.Message("helmenu: erro: " + err.Error())
			} else if msg != "" {
				action.InfoBar.Message(msg)
			}
			return true // consumiu
		}
	}

	helMenuActive = false
	action.InfoBar.Message("helmenu: cancelado")
	return true // consumiu
}

// ---- helpers de desenho com tcell ----

func drawBox(x, y, w, h int, title string) {
	hor := '─'
	ver := '│'
	tl, tr, bl, br := '┌', '┐', '└', '┘'
	st := config.DefStyle

	// bordas
	for i := 0; i < w; i++ {
		screen.Screen.SetContent(x+i, y, hor, nil, st)
		screen.Screen.SetContent(x+i, y+h-1, hor, nil, st)
	}
	for j := 0; j < h; j++ {
		screen.Screen.SetContent(x, y+j, ver, nil, st)
		screen.Screen.SetContent(x+w+1, y+j, ver, nil, st) // +1 pra fechar a caixa
	}
	screen.Screen.SetContent(x, y, tl, nil, st)
	screen.Screen.SetContent(x+w+1, y, tr, nil, st)
	screen.Screen.SetContent(x, y+h-1, bl, nil, st)
	screen.Screen.SetContent(x+w+1, y+h-1, br, nil, st)

	// preencher dentro com espaços
	for yy := y + 1; yy < y+h-1; yy++ {
		for xx := x + 1; xx < x+w+1; xx++ {
			screen.Screen.SetContent(xx, yy, ' ', nil, st)
		}
	}
	// título
	putString(x+1, y, " "+padCenter(title, w-1)+" ", st)
}

func putString(x, y int, s string, st tcell.Style) {
	for i, r := range s {
		screen.Screen.SetContent(x+i, y, r, nil, st)
	}
}

func padRight(s string, w int) string {
	if len([]rune(s)) >= w {
		return string([]rune(s)[:w])
	}
	return s + string(make([]rune, w-len([]rune(s))))
}
func padCenter(s string, w int) string {
	rs := []rune(s)
	if len(rs) >= w {
		return string(rs[:w])
	}
	left := (w - len(rs)) / 2
	right := w - len(rs) - left
	return string(make([]rune, left)) + s + string(make([]rune, right))
}
