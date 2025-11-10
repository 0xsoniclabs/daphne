# Daphne Analysis

This directory contains a collection of Jupyter Notebooks for analyzing data produced by simulation runs of the Daphne tool.


## Getting Started

Please see the official [Jupyter Notebook in VS Code](https://code.visualstudio.com/docs/datascience/jupyter-notebooks) introduction page for a brief overview of Jupyter Notebook concepts. In particular, the offered 6 minute introduction [video](https://youtu.be/suAkMeWJ1yE) can help familiarize oneself with key concepts.


## Intended Usage
The notebooks are intended to be used within the VS Code editor using the official Jupyter Notebook [extension](https://marketplace.visualstudio.com/items?itemName=ms-toolsai.jupyter).

A typical use case would look as follows:
- Run a Daphne simulation in the projects main directory using `go run ./daphne run`
- The simulation run produces an `output.csv` file with all the collected data in the projects root directory
- Open an analysis notebook in this directory and run all cells in the notebook to refresh the report
- Use the VS Code editor and Jupyter Notebook extension features to explore, refine, or export reports

If you are extending or modifying analyses that are of common use to others, consider committing these changes to the shared Daphne repository via a Pull Request.


> **_NOTE:_** Notebook files contain temporary report results. These can be quite large. Thus, to avoid bloat on the git repository, please make sure to purge notebook files from temporary results before including them in git commits. Look for the `Clear all Outputs` button in the VS editor.

## Installation

You need the following extensions in VS Code (both included in the projects plug-in recommendation list):
- [Jupyter](https://marketplace.visualstudio.com/items?itemName=ms-toolsai.jupyter) - for the notebook editor support
- [Python](https://marketplace.visualstudio.com/items?itemName=ms-python.python) - for Python syntax highlighting and other editor features

Furthermore, you have to install Jupyter notebook itself as well as a few key python libraries for performing analysis and rendering figures on your system. 
The recommendation is to set up a Python3 [virtual environment](https://docs.python.org/3/library/venv.html) -- either manually or through the Jupyter notebook plug-in in Visual Code. In this environment, the following packages should be installed:
```
pip install pandas numpy seaborn pyarrow fastparquet dask
```

To install all libraries in a verified version, run the following command:
```
pip install -r requirements.txt
```

**Optionally:** To support the export of notebooks to HTML or PDF, consider installing the Notebook conversion tool using
```
pip install nbconvert
```

## Additional Links

A list of additional reference material for further information:
- a gallery of example charts supported by the seaborn library can be found [here](https://seaborn.pydata.org/examples/index.html)
- the Numpy documentation can be found [here](https://numpy.org/doc/)
- the Pandas user guide can be found [here](https://pandas.pydata.org/docs/user_guide/index.html#user-guide)


## Open Tasks and Known Issues

The following tasks are pending for this directory:
- provisioning of a Docker file with the required dependencies
- provisioning of Docker based support for the scriptable rendering of reports
- look for improved tooling for performing code reviews on notebooks
- CI coverage for notebooks
