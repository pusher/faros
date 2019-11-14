# namespacecheck

namespacecheck checks whether a given Faros setup
is valid within the ownership rules added in v0.7.
See the [migration to v7.0](https://pusher.github.io/faros/migrate) docs for more information

To run the tool, grab 3 files off your kubernetes cluster

```
# kubectl get gittracks --all-namespaces -o json > gt.json
# kubectl get gittrackobjects --all-namespaces -o json > gto.json
# kubectl get clustergittrackobjects -all-namespaces -o json > cgto.json
```

Then run
```
go run github.com/pusher/faros/hack/namespacecheck --gt-file gt.json --gto-file gto.json --cgto-file cgto.json
```

The tool will report back if you have any cross namespace ownership
and write a file containing any new clustergittracks that will need to
be created
