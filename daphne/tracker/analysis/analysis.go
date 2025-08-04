package analysis

import (
	"errors"
	"os"

	"github.com/0xsoniclabs/daphne/daphne/tracker"
)

const (
	TxCreated   = "TxCreated"
	TxProcessed = "TxProcessed"
)

func ProduceReport(
	data []tracker.Entry,
	outputDirectory string,
) (string, error) {

	file, err := createTempFile(".csv")
	if err != nil {
		return "", err
	}
	defer func() {
		err = errors.Join(err, os.Remove(file))
	}()
	out, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return "", err
	}
	defer func() {
		err = errors.Join(err, out.Close())
	}()
	if err := exportToCSV(data, out); err != nil {
		return "", err
	}

	return EvalReport.Render(file, outputDirectory)
}
