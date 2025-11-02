//go:build helmenu

// helmenu_demo.go
// PT-BR: Adiciona um menu próprio (Alt-M) com 3 funcionalidades de exemplo.
// EN: Adds a custom menu (Alt-M) with 3 example actions.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

// ---------------------------
// Configuração do seu menu
// ---------------------------

type MenuItem struct {
	Key   string                         // tecla de atalho no menu (1,2,3,...)
	Title string                         // rótulo que aparece no menu
	Run   func() (msg string, err error) // ação executada ao escolher
}

// (PT) Suas ações: coloque aqui suas funcionalidades reais.
// (EN) Your actions: put your real features here.
var helMenuItems = []MenuItem{
	{
		Key:   "1",
		Title: "HEL-1: Dizer 'estou vivo!'",
		Run: func() (string, error) {
			return "HEL-1 executado: estou vivo! ✔", nil
		},
	},
	{
		Key:   "2",
		Title: "HEL-2: Inserir timestamp",
		Run: func() (string, error) {
			return "HEL-2 executado: " + time.Now().Format(time.RFC3339), nil
		},
	},
	{
		Key:   "3",
		Title: "HEL-3: Rodar tarefa custom",
		Run: func() (string, error) {
			// Coloque aqui sua lógica
			return "HEL-3 executado: tarefa custom concluída.", nil
		},
	},
}

// ---------------------------
// Desenho “cru” do menu no terminal
// ---------------------------

// (PT) Desenha uma caixa simples com opções; captura 1 tecla e retorna a escolhida.
// (EN) Draws a simple box with options; reads 1 key and returns chosen key.
func showHelMenuAndReadKey() (string, error) {
	// Para ser independente, usamos somente ANSI + leitura bruta de stdin.
	// Este é um overlay rápido; quando você integrar com a camada de UI do micro,
	// troque por chamadas de desenho/input do próprio editor.

	// Salvar estado do terminal; entrar em modo raw.
	restore, err := enterRawMode()
	if err != nil {
		return "", err
	}
	defer restore()

	// Limpa área do menu (canto superior direito, por exemplo) e desenha
	writeANSI("\x1b7")   // save cursor
	writeANSI("\x1b[H")  // home
	writeANSI("\x1b[2J") // clear screen (simples; pode trocar por área parcial)
	box := renderMenuBox(helMenuItems)
	writeANSI(box)
	writeANSI("\x1b8") // restore cursor

	// Ler 1 byte (tecla); para teclas especiais você pode expandir a lógica
	ch := make([]byte, 4)
	n, _ := os.Stdin.Read(ch)
	if n <= 0 {
		return "", fmt.Errorf("no key")
	}
	r, _ := utf8.DecodeRune(ch[:n])
	key := string(r)

	// Limpar overlay (redesenhar editor virá por cima de qualquer forma)
	// Aqui apenas envia uma limpeza rápida
	writeANSI("\x1b[2J")

	return key, nil
}

func renderMenuBox(items []MenuItem) string {
	var b strings.Builder
	// Uma “caixa” de 40 colunas, 2 linhas de margem
	// Você pode ajustar para alinhar onde quiser (centro, direita, etc.)
	title := " Hel Menu (Alt-M) "
	b.WriteString("\n")
	b.WriteString("+" + strings.Repeat("-", 40) + "+\n")
	b.WriteString("|" + padCenter(title, 40) + "|\n")
	b.WriteString("+" + strings.Repeat("-", 40) + "+\n")
	for _, it := range items {
		line := fmt.Sprintf("[%s] %s", it.Key, it.Title)
		b.WriteString("|" + padRight(line, 40) + "|\n")
	}
	b.WriteString("+" + strings.Repeat("-", 40) + "+\n")
	b.WriteString("|" + padRight("ESC/Outra tecla: cancelar", 40) + "|\n")
	b.WriteString("+" + strings.Repeat("-", 40) + "+\n")
	b.WriteString("\n")
	return b.String()
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	return s + strings.Repeat(" ", w-len(s))
}

func padCenter(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	left := (w - len(s)) / 2
	right := w - len(s) - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func writeANSI(s string) {
	_, _ = os.Stdout.Write([]byte(s))
}

// ---------------------------
// Integração com o “core” do micro
// ---------------------------

// (PT) Você vai: (a) chamar registerHelMenuAction() em algum ponto de init()
// do editor, e (b) mapear Alt-M para helmenu:open no seu despachante de teclas.
// (EN) You will: (a) call registerHelMenuAction() somewhere in editor init(),
// and (b) map Alt-M to helmenu:open in your key dispatcher.

func registerHelMenuAction() {
	// [INTEGRAR] 1) REGISTRE UMA AÇÃO/COMANDO
	// Exemplo conceitual:
	//   action.Register("helmenu:open", func(EditorContext) {
	//       openHelMenu()
	//   })
	// Adapte ao ponto real do seu fork (onde outras ações são registradas).
}

func openHelMenu() {
	key, err := showHelMenuAndReadKey()
	if err != nil {
		showStatus("helmenu: erro ao abrir menu: " + err.Error())
		return
	}
	for _, it := range helMenuItems {
		if key == it.Key {
			msg, runErr := it.Run()
			if runErr != nil {
				showStatus("helmenu: erro: " + runErr.Error())
			} else {
				showStatus(msg)
			}
			return
		}
	}
	showStatus("helmenu: cancelado")
}

// (PT) Mostra mensagem na barra de status do micro (substitua pela call real).
// (EN) Show a message on micro’s status bar (replace with the real call).
func showStatus(msg string) {
	// [INTEGRAR] 2) ESCREVER NA STATUS BAR
	// Troque pela função real do seu fork (ex.: editor.SetStatus(msg))
	_, _ = os.Stderr.WriteString("[status] " + msg + "\n")
}

// ---------------------------
// Modo raw minimalista
// ---------------------------

func enterRawMode() (restore func() error, err error) {
	// Colocar terminal em “raw” de forma segura e restaurar ao sair.
	// Para simplicidade, usamos stty via syscall; em produção, prefira uma lib de TTY.
	oldState, err := getSttyState()
	if err != nil {
		return nil, err
	}

	if err := stty("-icanon", "-echo", "min", "1"); err != nil {
		return nil, err
	}

	// Restaurar ao receber SIGINT/SIGTERM também
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		_ = stty(oldState...)
		os.Exit(0)
	}()

	return func() error { return stty(oldState...) }, nil
}

func getSttyState() ([]string, error) {
	out, err := runCmdCapture("stty", "-g")
	if err != nil {
		return nil, err
	}
	return []string{out}, nil
}

func stty(args ...string) error {
	_, err := runCmdCapture("stty", args...)
	return err
}

func runCmdCapture(name string, args ...string) (string, error) {
	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()

	pid, err := syscall.ForkExec(name, append([]string{name}, args...), &syscall.ProcAttr{
		Files: []uintptr{os.Stdin.Fd(), w.Fd(), os.Stderr.Fd()},
	})
	if err != nil {
		return "", err
	}
	_, _ = syscall.Wait4(pid, nil, 0, nil)

	buf := make([]byte, 1<<16)
	n, _ := r.Read(buf)
	return strings.TrimSpace(string(buf[:n])), nil
}
