
```txt
____    __    ____  ___   .___________.  ______  __    __                 _______   __   _______  _______ 
\   \  /  \  /   / /   \  |           | /      ||  |  |  |      ___      |       \ |  | |   ____||   ____|
 \   \/    \/   / /  ^  \ |---|  |----||  |---/ |  |__|  |     ( _ )     |  .--.  ||  | |  |__   |  |__   
  \            / /  /_\  \    |  |     |  |     |   __   |     / _ \/\   |  |  |  ||  | |   __|  |   __|  
   \    /\    / /  _____  \   |  |     |  \----.|  |  |  |    | (_>  <   |  '--'  ||  | |  |     |  |
    \__/  \__/ /__/     \__\  |__|      \______||__|  |__|     \___/\/   |_______/ |__| |__|     |__|
```

# Notes

A kubectl plugin for watching resources and generating diffs.

If you want to watch multiple objects, and some of them are namespace-scoped, then they must be in the same namespace.

# Prerequisite

This tool need `diff` utility for files comparison, so make sure `diffutils` has already been installed.
> GNU Diffutils: https://www.gnu.org/software/diffutils/

If you want a colorful output, you can install `colordiff` wrapper for `diff` tool.
> Colordiff: https://www.colordiff.org/

# Install

1. You can download the latest release version from:
    > https://github.com/leopoldxx/kube-watch-diff/releases

# Usage

You could use it like the examples below（install as kubectl plugin):

```shell
# watch a namespace scoped resource(use without kubectl)
kubectl-watch pod pod1
```
```shell
# watch a clusters scoped resource 
kubectl watch node node1
```
```shell
# watch multiple resources in a same namespace
kubectl watch nodes/node1  pods/pod1 pods/pod2
```
```shell
# watch multiple resources using a label selector
kubectl watch pods -l far=bar
kubectl watch deployment,rs -l far=bar
```
```shell
# watch all pods on the same node
kubectl watch pods --field-selector spec.nodeName=192.168.1.1
```
```shell
# watch 'all' category resource using label selector
kubectl watch all -l far=bar -n test-ns
```
```shell
# watch all masters
kubectl watch node -l node-role.kubernetes.io/master=""
```
```shell
# watch all nodes, and record the diffs into a file
kubectl-watch nodes --all 2>/dev/null | tee nodes.diff
```
