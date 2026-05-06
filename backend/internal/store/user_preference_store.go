package store

type UserPreferenceStore interface {
	Get(userID, key string) (string, bool, error)
	Set(userID, key, value string) error
}

