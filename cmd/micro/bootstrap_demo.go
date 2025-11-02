//go:build demo_estou_vivo

// Português: Este arquivo injeta um arquivo temporário com o texto "estou vivo!"
// no fluxo de inicialização do micro, adicionando-o em os.Args antes do main().
// English: This file injects a temp file with "estou vivo!" into micro's startup
// by appending it to os.Args before main() runs.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// makeTempFile creates a temp file with given contents and returns its absolute path.
// (PT-BR) Cria um arquivo temporário com o conteúdo indicado e retorna o caminho absoluto.
func makeTempFile(contents string) (string, error) {
	dir := os.TempDir()
	name := fmt.Sprintf("micro_boot_%d.txt", time.Now().UnixNano())

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// init() roda antes de main(). Aqui nós criamos o arquivo e o injetamos nos argumentos.
// (EN) init() runs before main(). We create the file and inject it into os.Args here.
func init() {
	// Você pode trocar a mensagem aqui à vontade:
	// (EN) Feel free to change the message here:
	const msg = "estou vivo!\n"

	p, err := makeTempFile(msg)
	if err != nil {
		// Não falhe o editor se algo der errado; apenas continue sem injeção.
		// (EN) Don’t fail the editor; just continue without injection.
		fmt.Fprintf(os.Stderr, "[micro demo] warning: could not create temp file: %v\n", err)
		return
	}

	// Se o usuário já passou arquivos por argumento, mantemos tudo e só acrescentamos o nosso.
	// (EN) If user passed files, keep them and just append ours.
	os.Args = append(os.Args, p)

	// Dica de debug opcional:
	// (EN) Optional debug tip:
	// fmt.Fprintf(os.Stderr, "[micro demo] injected temp file: %s\n", p)
}
