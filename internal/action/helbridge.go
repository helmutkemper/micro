package action

// Bridge para executar comandos do mapa `commands` sem abrir o prompt,
// localizando o *BufPane via MainTab().Panes (varrendo panes).
//
// Ex.: HelRunCmd("hsplit"), HelRunCmd("vsplit"), HelRunCmd("tab", "README.md")

import (
	"errors"
	"fmt"
	"strings"
)

// tenta identificar “foco” sem acoplar em nomes exatos (forks variam)
type paneHasFocus interface{ HasFocus() bool }
type paneIsActive interface{ IsActive() bool }

// devolve o *BufPane ativo, ou o primeiro *BufPane encontrado
func helActiveBufPane() (*BufPane, error) {
	mt := MainTab()
	if mt == nil {
		return nil, errors.New("MainTab()==nil")
	}

	// 1) prioriza o pane focado/ativo
	for _, p := range mt.Panes {
		if bp, ok := p.(*BufPane); ok && bp != nil {
			// se a interface de foco existir, respeite-a
			if x, ok := p.(paneHasFocus); ok && x.HasFocus() {
				return bp, nil
			}
			if x, ok := p.(paneIsActive); ok && x.IsActive() {
				return bp, nil
			}
		}
	}
	// 2) se ninguém reporta foco, pegue o primeiro *BufPane
	for _, p := range mt.Panes {
		if bp, ok := p.(*BufPane); ok && bp != nil {
			return bp, nil
		}
	}

	return nil, errors.New("nenhum *BufPane encontrado em MainTab().Panes")
}

// HelRunCmd executa um comando (como no Ctrl-E), sem abrir prompt.
// name: "hsplit", "vsplit", "tab", ...; args: argumentos (já tokenizados)
func HelRunCmd(name string, args ...string) error {
	if commands == nil {
		return errors.New("commands==nil (InitCommands() não foi chamado?)")
	}
	cmd, ok := commands[name]
	if !ok {
		return fmt.Errorf("comando não encontrado: %s", name)
	}
	if cmd.action == nil {
		return fmt.Errorf("comando %q sem função", name)
	}

	// Se precisamos de um *BufPane, localize-o (todos os seus comandos são (*BufPane).XxxCmd)
	bp, err := helActiveBufPane()
	if err != nil {
		// Se não há nenhum BufPane aberto ainda, crie uma aba vazia e tente de novo.
		// (O próprio comando "tab" também é (*BufPane).NewTabCmd, então precisamos
		// de um pane mínimo. Neste caso, criamos um temporário via open "".)
		//
		// Use o driver existente: muitos forks suportam open "" para criar buffer vazio;
		// se no seu fork não for assim, troque por algo válido (ex.: "open untitled")
		if name != "open" {
			// tente abrir um buffer vazio e então reexecutar
			if err2 := HelRunCmd("open", ""); err2 != nil {
				return fmt.Errorf("sem *BufPane e falhou abrir buffer: %v; erro original: %w", err2, err)
			}
			bp, err = helActiveBufPane()
			if err != nil {
				return fmt.Errorf("buffer criado mas ainda sem *BufPane: %w", err)
			}
		} else {
			return fmt.Errorf("sem *BufPane para executar %q: %w", name, err)
		}
	}

	// assinatura: func(*BufPane, []string)
	cmd.action(bp, args)
	return nil
}

// HelRunCmdLine aceita uma linha como no prompt e executa.
// Ex.: HelRunCmdLine("tab README.md")
func HelRunCmdLine(line string) error {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	parts := strings.Fields(line)
	return HelRunCmd(parts[0], parts[1:]...)
}
