package store

import "encoding/json"

type RBACPolicyStore interface {
	Get() (json.RawMessage, bool, error)
	Set(policy json.RawMessage) error
}
