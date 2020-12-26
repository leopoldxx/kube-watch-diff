
```txt
____    __    ____  ___   .___________.  ______  __    __                 _______   __   _______  _______ 
\   \  /  \  /   / /   \  |           | /      ||  |  |  |      ___      |       \ |  | |   ____||   ____|
 \   \/    \/   / /  ^  \ |---|  |----||  |---/ |  |__|  |     ( _ )     |  .--.  ||  | |  |__   |  |__   
  \            / /  /_\  \    |  |     |  |     |   __   |     / _ \/\   |  |  |  ||  | |   __|  |   __|  
   \    /\    / /  _____  \   |  |     |  \----.|  |  |  |    | (_>  <   |  '--'  ||  | |  |     |  |
    \__/  \__/ /__/     \__\  |__|      \______||__|  |__|     \___/\/   |_______/ |__| |__|     |__|
```

# Notes

A kubectl plugin for watching resource diff.

If you want to watch multiple objects, and some of them are namespace-scoped, then they must be in the same namespace.

As a kubectl plugin, you could call this command like 'kubectl watch', only if you install kubectl-watch into system PATH.

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
```
```shell
# watch 'all' category resource using label selector
kubectl watch all -l far=bar -n test-ns
```
```shell
# watch all masters
kubectl watch node -l node-role.kubernetes.io/master=""
```
