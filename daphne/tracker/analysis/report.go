package analysis

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Report struct {
	name     string
	template []byte
}

//go:embed eval_report.Rmd
var evalReportTemplate []byte

// A list of predefined reports.
var (
	EvalReport = Report{
		name:     "eval_report",
		template: evalReportTemplate,
	}
)

//go:embed render.R
var renderScript []byte

func (r *Report) Render(datafile, outputDirectory string) (
	report string,
	err error,
) {
	// Create a temporary file for the R script creating the report.
	script, err := createTempFile(".R")
	if err != nil {
		return "", err
	}
	defer func() {
		err = errors.Join(err, os.Remove(script))
	}()
	if err := os.WriteFile(script, renderScript, 0644); err != nil {
		return "", err
	}

	// Create a temporary file for the R markdown template of the report.
	template, err := createTempFile(".Rmd")
	if err != nil {
		return "", err
	}
	defer func() {
		err = errors.Join(err, os.Remove(template))
	}()
	if err := os.WriteFile(template, r.template, 0644); err != nil {
		return "", err
	}

	// Run the R script to generate the report.
	report = r.name + ".html"
	cmd := exec.Command("Rscript", script, template, datafile, outputDirectory, report)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%v\n%v", out.String(), err)
	}

	return filepath.Join(outputDirectory, report), nil
}

func createTempFile(suffix string) (string, error) {
	file, err := os.CreateTemp("", "tmp_*"+suffix)
	if err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return file.Name(), nil
}
