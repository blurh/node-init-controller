package config

import (
    "github.com/spf13/viper"
    corev1 "k8s.io/api/core/v1"
)

type Config struct {
    InitScript      string  `yaml:"initScript"`
    Labels          []Label `yaml:"labels"`
    Taints          []Taint `yaml:"taints"`
    Registry        string  `yaml:"registry"`
    ImagePullSecret string  `yaml:"imagePullSecret"`
}

type Label struct {
    Key   string `yaml:"key"`
    Value string `yaml:"value"`
}

type Taint struct {
    Key    string             `yaml:"key"`
    Value  string             `yaml:"value"`
    Effect corev1.TaintEffect `yaml:"effect"`
}

func NewConfig() *Config {
    vc := viper.New()
    vc.SetConfigFile("config/config.yaml")
    vc.ReadInConfig()
    c := &Config{}
    _ = vc.Unmarshal(c)
    return c
}

func (c *Config) reload() {
    *c = *NewConfig()
}

func (c *Config) GetInitScript() string {
    c.reload()
    return c.InitScript
}

func (c *Config) GetLabels() []Label {
    c.reload()
    return c.Labels
}

func (c *Config) GetTaints() []Taint {
    c.reload()
    return c.Taints
}

func (c *Config) GetRegistry() string {
    c.reload()
    return c.Registry
}

func (c *Config) GetImagePullSecret() string {
    c.reload()
    return c.ImagePullSecret
}
