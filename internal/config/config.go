package config

var C Config

type Config struct {
	Version  string
	LogLevel int       `mapstructure:"logLevel"`
	Mode     string    `mapstructure:"mode"`
	Addr     string    `mapstructure:"addr"`
	Conf     string    `mapstructure:"conf"`
	TLS      TLSConfig `mapstructure:"tls"`
}

type TLSConfig struct {
	ForwardSecurity bool          `mapstructure:"fs"`
	Certs           []TLSCertPair `mapstructure:"certs"`
}

type TLSCertPair struct {
	Cert string `mapstructure:"cert"`
	Key  string `mapstructure:"key"`
}

type ServerConf struct {
	SNI   string `json:"sni"`
	Mode  string `json:"mode"`
	Addr  string `json:"addr"`
	Auth  string `json:"auth"`
	Mux   bool   `json:"mux"`
	MuxV2 bool   `json:"muxV2"`
}

func (s *ServerConf) ConnMode() string {
	if s.Mux {
		return "mux-" + s.Mode
	}
	return s.Mode
}

type AgentConf struct {
	SNI     string `json:"sni"`
	Remote  string `json:"remote"`
	Local   string `json:"local"`
	Auth    string `json:"auth"`
	Mux     bool   `json:"mux"`
	Pool    bool   `json:"pool"`
	MaxIdle int    `json:"maxIdle"`
	MaxMux  int    `json:"maxMux"`
	MuxV2   bool   `json:"muxV2"`
}

func (a *AgentConf) ConnMode() string {
	if a.Mux {
		if a.Pool {
			return "mux-pool"
		}
		return "mux-tcp"
	}
	return "tcp"
}
