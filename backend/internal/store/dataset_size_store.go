package store

import "time"

// DatasetSize is what one dataset held, the last time anybody looked.
//
// Measured out of band rather than while somebody waits for a page. Walking a
// multi-gigabyte bucket inline is why the home page carried a deadline, gave
// up halfway on large datasets and skipped regulated ones altogether - a
// figure partial by construction, which could not say how partial. Once a
// night it costs nobody anything and the total is complete.
type DatasetSize struct {
	DatasetID  string
	Bytes      int64
	Objects    int64
	MeasuredAt time.Time
	// Failure is why this dataset could not be measured, empty when it could.
	//
	// Recorded rather than omitted: a dataset nobody has measured yet and one
	// whose credentials stopped working are different facts, and a missing row
	// would make them look identical.
	Failure string
}

type DatasetSizeStore interface {
	Upsert(entry DatasetSize) error
	List() ([]DatasetSize, error)
}
