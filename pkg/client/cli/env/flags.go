package env

import (
	"github.com/spf13/pflag"
)

type Flags struct {
	File   string // --env-file
	Syntax Syntax // --env-syntax
	JSON   string // --env-json
}

func (a *Flags) AddFlags(flagSet *pflag.FlagSet) {
	flagSet.StringVarP(&a.File, "env-file", "e", "", ``+
		`Also emit the remote environment to an file. The syntax used in the file can be determined using flag --env-syntax`)

	flagSet.Var(&a.Syntax, "env-syntax", `Syntax used for env-file. One of `+SyntaxUsage())

	flagSet.StringVarP(&a.JSON, "env-json", "j", "", `Also emit the remote environment to a file as a JSON blob.`)
}

func (a *Flags) PerhapsWrite(env map[string]string) error {
	if a.File != "" {
		if err := a.Syntax.writeFile(a.File, env); err != nil {
			return err
		}
	}
	if a.JSON != "" {
		if err := SyntaxJSON.writeFile(a.JSON, env); err != nil {
			return err
		}
	}
	return nil
}
