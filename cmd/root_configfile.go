package cmd

import (
	"os"
	"text/template"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/mikumaycry/akari/internal/config"
)

const configTemplate = `{
	"LogLevel": {{.LogLevel}},
	"Mode": "{{.Mode}}",
	"Addr": "{{.Addr}}",
	"Conf": "{{.Conf}}",
	"TLS": {
		"ForwardSecurity": "{{.TLS.ForwardSecurity}}",
		"Certs": {{.TLS.Certs}}
	}
}
`

var configCmd = &cobra.Command{
	Use:   "configfile",
	Short: "Print akari configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		t := template.Must(template.New("config").Parse(configTemplate))
		err := t.Execute(os.Stdout, &config.C)
		if err != nil {
			return errors.Wrap(err, "execute config template")
		}
		return nil
	},
}
