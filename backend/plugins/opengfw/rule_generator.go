package opengfw

import (
	"lxdapi/models"
	"strings"
)

type RuleGeneratorV3 struct {
	config *models.FirewallConfig
}

func NewRuleGeneratorV3(config *models.FirewallConfig) *RuleGeneratorV3 {
	return &RuleGeneratorV3{config: config}
}

func (g *RuleGeneratorV3) Generate() (string, error) {
	rules := strings.TrimSpace(g.config.Rules)
	if rules == "" {
		return "[]", nil
	}
	return rules, nil
}
