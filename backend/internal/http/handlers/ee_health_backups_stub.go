//go:build !enterprise

package handlers

// Backup is an Enterprise capability. The Community build reports no backup
// alerts because it performs no backups: the health surface stays honest
// rather than announcing a subsystem that is not present.
func (h Handlers) backupAlerts() []healthAlert { return nil }
