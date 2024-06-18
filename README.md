# dependency-filter

Dfliter(dependency-filter) is a tool used to filter changes in Maven's local dependency repository

## Usage

* Build

```shell
go build -o dfilter.exe
```

* Run

    ```shell
    dfilter.exe --dependency [dependency_path] --mode [latest/compare] --interval [interval_time]
    ```

    * Explain
        * dependency: maven dependency path which need to be scanned
        * mode: 1)compare mode: compare old dependency list with the newly;2)latest mode: filter the latest modified
          time dependency
        * interval: filter time range(minutes) away from the latest modified time