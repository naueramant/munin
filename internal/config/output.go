package config

import (
	"io"

	"gopkg.in/yaml.v3"
)

const maskedSecret = "***"

// EncodeYAML writes v as YAML with two-space indentation.
func EncodeYAML(w io.Writer, v interface{}) error {
	if n, ok := v.(*yaml.Node); ok {
		clearMergeTags(n)
	}
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return enc.Close()
}

// clearMergeTags drops the explicit !!merge tag from "<<" keys so they render
// as plain "<<:" instead of "!!merge <<:".
func clearMergeTags(n *yaml.Node) {
	if n.Kind == yaml.ScalarNode && n.Tag == "!!merge" {
		n.Tag = ""
	}
	for _, child := range n.Content {
		clearMergeTags(child)
	}
}

// MaskSecrets replaces every non-empty auth.password value in the tree with "***".
func MaskSecrets(n *yaml.Node) {
	if n == nil {
		return
	}
	if n.Kind == yaml.MappingNode {
		for i := 1; i < len(n.Content); i += 2 {
			key, value := n.Content[i-1], n.Content[i]
			if key.Value == "auth" && value.Kind == yaml.MappingNode {
				for j := 1; j < len(value.Content); j += 2 {
					if value.Content[j-1].Value == "password" && value.Content[j].Kind == yaml.ScalarNode && value.Content[j].Value != "" {
						value.Content[j].Value = maskedSecret
						value.Content[j].Tag = "!!str"
					}
				}
			}
		}
	}
	for _, child := range n.Content {
		MaskSecrets(child)
	}
}

// MaskSecrets replaces every non-empty auth password with "***".
func (c *Configuration) MaskSecrets() {
	for i := range c.Tabs {
		c.Tabs[i].Auth.mask()
	}
	for i := range c.Schedules {
		c.Schedules[i].Auth.mask()
	}
}

func (a *Auth) mask() {
	if a.Password != "" {
		a.Password = maskedSecret
	}
}
