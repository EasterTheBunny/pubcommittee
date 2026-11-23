package adminapi

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Operator is one administrator of this environment: a stable id, a public
// name, and the verified principal that identifies them here. The principal
// is private: it is never logged, published, or sent to the log.
type Operator struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Principal string `json:"principal"`
}

// Config is the operator list of one environment, produced from the private
// operator roster by infrastructure code.
type Config struct {
	Operators []Operator `json:"operators"`
	// DeployPrincipal is the environment's deploy service account. It may call
	// the roster route and nothing else.
	DeployPrincipal string `json:"deploy_principal"`
}

// LoadConfig decodes and validates a Config. Unknown fields are an error.
func LoadConfig(r io.Reader) (Config, error) {
	var c Config
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("operator list: %w: %w", data.ErrInvalid, err)
	}
	return c, c.Validate()
}

// Validate checks that ids, names, and principals are present and unique, and
// that the deploy principal is not also an operator.
func (c Config) Validate() error {
	ids, principals := map[string]bool{}, map[string]bool{}
	for _, o := range c.Operators {
		switch {
		case strings.TrimSpace(o.ID) == "", strings.TrimSpace(o.Name) == "", strings.TrimSpace(o.Principal) == "":
			return fmt.Errorf("operator list: every operator needs an id, a name and a principal: %w", data.ErrInvalid)
		case ids[o.ID]:
			return fmt.Errorf("operator list: duplicate operator id %q: %w", o.ID, data.ErrInvalid)
		case principals[o.Principal]:
			return fmt.Errorf("operator list: two operators share a principal: %w", data.ErrInvalid)
		}
		ids[o.ID], principals[o.Principal] = true, true
	}
	if principals[c.DeployPrincipal] {
		return fmt.Errorf("operator list: the deploy principal is also an operator: %w", data.ErrInvalid)
	}
	return nil
}

// Role is what a verified principal may do.
type Role int

// Roles.
const (
	// RoleNone is a principal that is not on the list, even if it holds IAM access.
	RoleNone Role = iota
	// RoleOperator may call every route except the roster route.
	RoleOperator
	// RoleDeploy may call the roster route only.
	RoleDeploy
)

// Identify maps a verified principal to its role and, for an operator, the operator.
func (c Config) Identify(principal string) (Role, Operator) {
	if principal == "" {
		return RoleNone, Operator{}
	}
	if principal == c.DeployPrincipal {
		return RoleDeploy, Operator{}
	}
	for _, o := range c.Operators {
		if o.Principal == principal {
			return RoleOperator, o
		}
	}
	return RoleNone, Operator{}
}

func (o Operator) logActor() *data.LogOperator {
	return &data.LogOperator{ID: o.ID, Name: o.Name}
}
