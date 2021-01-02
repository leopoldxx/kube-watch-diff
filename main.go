/*
Copyright © 2020 leopoldxx

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/go-yaml/yaml"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	cliflag "k8s.io/component-base/cli/flag"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

var longDesc = `A kubectl plugin for watching resource diff.
if you want to watch multi objects, and some of them are namespace-scoped.
then they must be in the same namespace.
As a kubectl plugin, you could call this command like 'kubectl watch', only if you put kubectl-watch into system PATH.
`

var logo = `
____    __    ____  ___   .___________.  ______  __    __                 _______   __   _______  _______ 
\   \  /  \  /   / /   \  |           | /      ||  |  |  |      ___      |       \ |  | |   ____||   ____|
 \   \/    \/   / /  ^  \ |---|  |----||  |---/ |  |__|  |     ( _ )     |  .--.  ||  | |  |__   |  |__   
  \            / /  /_\  \    |  |     |  |     |   __   |     / _ \/\   |  |  |  ||  | |   __|  |   __|  
   \    /\    / /  _____  \   |  |     |  \----.|  |  |  |    | (_>  <   |  '--'  ||  | |  |     |  |
    \__/  \__/ /__/     \__\  |__|      \______||__|  |__|     \___/\/   |_______/ |__| |__|     |__|

`

var examples = `# watch a namespace scoped resource
kubectl-watch pod pod1

# watch a clusters scoped resource 
kubectl watch node node1

# watch multiple resources in a same namespace
kubectl watch nodes/node1  pods/pod1 pods/pod2

# watch multiple resources using a label selector
kubectl watch pods -l far=bar

# watch 'all' category resource using label selector
kubectl watch all -l far=bar -n test-ns

# watch all masters
kubectl watch node -l node-role.kubernetes.io/master=""

# watch all nodes, and record the diff into a file
kubectl-watch nodes --all 2>/dev/null | tee nodes.diff
`

var (
	Version   string
	GoVersion string
	Branch    string
	Commit    string
	BuildTime string
)

func ShowVersion(w io.Writer) {
	if Version == "" {
		return
	}
	fmt.Fprintf(w, `build time: %s
%s: %s
%s
git: %s, %s
`,
		BuildTime,
		path.Base(os.Args[0]), Version,
		GoVersion,
		Branch, Commit,
	)
}

var onlyOneSignalHandler = make(chan struct{})

func setupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler)
	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1)
	}()
	return stop
}

func main() {
	closeCtx, cancel := context.WithCancel(context.Background())
	go func() {
		<-setupSignalHandler()
		cancel()
	}()

	root := newWatchDiffCommand(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	err := root.ExecuteContext(closeCtx)
	if err != nil {
		log.Fatal(err)
	}
}

type watchDiffOptions struct {
	RESTClientGetter     genericclioptions.RESTClientGetter
	ResourceBuilderFlags *genericclioptions.ResourceBuilderFlags
	Timeout              time.Duration
	genericclioptions.IOStreams
}

func (options *watchDiffOptions) AddFlags(cmd *cobra.Command) {
	options.ResourceBuilderFlags.AddFlags(cmd.Flags())

	cmd.Flags().DurationVarP(&options.Timeout, "timeout", "t", options.Timeout,
		"The length of time to watch before giving up.")
}

func (options *watchDiffOptions) ToCommand(args []string) (*watchDiffCommand, error) {
	builder := options.ResourceBuilderFlags.ToBuilder(options.RESTClientGetter, args)
	clientConfig, err := options.RESTClientGetter.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	effectiveTimeout := options.Timeout
	if effectiveTimeout <= 0 {
		effectiveTimeout = time.Hour
	}
	o := &watchDiffCommand{
		ResourceFinder: builder,
		DynamicClient:  dynamicClient,
		Timeout:        effectiveTimeout,
		IOStreams:      options.IOStreams,
	}
	return o, nil
}

func newWatchDiffOptions(restClientGetter genericclioptions.RESTClientGetter,
	streams genericclioptions.IOStreams) *watchDiffOptions {
	return &watchDiffOptions{
		RESTClientGetter: restClientGetter,
		ResourceBuilderFlags: genericclioptions.NewResourceBuilderFlags().
			WithLabelSelector("").
			WithFieldSelector("").
			WithAll(false).
			WithAllNamespaces(false).
			WithLocal(false).
			WithLatest(),
		Timeout:   10 * time.Minute,
		IOStreams: streams,
	}
}

type watchDiffCommand struct {
	genericclioptions.IOStreams
	ResourceFinder genericclioptions.ResourceFinder
	DynamicClient  dynamic.Interface
	Timeout        time.Duration
}

func newWatchDiffCommand(streams genericclioptions.IOStreams) *cobra.Command {
	var showVersion bool
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	options := newWatchDiffOptions(cmdutil.NewFactory(kubeConfigFlags), streams)
	cmd := &cobra.Command{
		Use:     "kubectl-watch [resources] [name]",
		Short:   "kubectl-watch\n A kubectl plugin for watching resource diff",
		Long:    longDesc,
		Example: examples,
		Run: func(cmd *cobra.Command, args []string) {
			ShowVersion(streams.ErrOut)
			if showVersion {
				return
			}
			_, _ = fmt.Fprintln(streams.ErrOut, logo)
			wdcmd, err := options.ToCommand(args)
			cmdutil.CheckErr(err)
			err = wdcmd.Run(cmd.Context())
			cmdutil.CheckErr(err)
		},
	}

	flags := cmd.PersistentFlags()
	flags.SetNormalizeFunc(cliflag.WarnWordSepNormalizeFunc)
	flags.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	options.AddFlags(cmd)

	kubeConfigFlags.AddFlags(flags)
	flags.BoolVar(&showVersion, "version", showVersion, "only show version info")
	return cmd
}

func (o *watchDiffCommand) Run(ctx context.Context) error {
	err := o.ResourceFinder.Do().Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return nil
		}
		go func() {
			if o.Timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, o.Timeout)
				defer cancel()
			}

			var defaultResync time.Duration
			nameSelector := fields.OneTermEqualSelector("metadata.name", info.Name).String()
			f := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
				o.DynamicClient,
				defaultResync,
				info.Namespace,
				func(options *metav1.ListOptions) {
					options.FieldSelector = nameSelector
				},
			)

			dobj := diffobj{
				resource:  info.Mapping.Resource.Resource,
				namespace: info.Namespace,
				name:      info.Name,
				output:    o.Out,
			}
			informer := f.ForResource(info.Mapping.Resource)
			informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					dobj.diffWithPrevious(obj)
				},
				UpdateFunc: func(_, newObj interface{}) {
					dobj.diffWithPrevious(newObj)
				},
				DeleteFunc: func(obj interface{}) {
					dobj.diffWithPrevious(obj)
				},
			})
			fmt.Fprintf(
				o.ErrOut,
				"start watching the diff of %s on %s @Now=%s\n",
				info.Mapping.Resource.Resource,
				func() string {
					if info.Namespace != "" {
						return info.Namespace + "/" + info.Name
					}
					return info.Name
				}(),
				time.Now(),
			)
			informer.Informer().Run(ctx.Done())
		}()
		return nil
	})
	if err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}

type diffobj struct {
	resource   string
	namespace  string
	name       string
	output     io.Writer
	baseObject *unstructured.Unstructured
	baseYAML   string
}

func (o *diffobj) fileName() string {
	return o.formatName("_")
}
func (o *diffobj) labelName() string {
	return o.formatName("/") + ".yaml"
}
func (o *diffobj) formatName(sep string) string {
	if o.namespace == "" {
		return o.resource + sep + o.name // cluster scoped resource
	}
	return o.resource + sep + o.namespace + sep + o.name // namespace scoped resource
}

func (o *diffobj) diffWithPrevious(obj interface{}) {
	unstObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return
	}
	d, _ := yaml.Marshal(obj)
	if o.baseObject == nil {
		o.baseObject = unstObj
		o.baseYAML = string(d)
		return
	}
	// ignore if the new coming obj is the same as previous one
	if o.baseObject.GetResourceVersion() == unstObj.GetResourceVersion() {
		return
	}
	err := o.diff(o.baseYAML, string(d))
	if err != nil {
		log.Println("diff failed", err)
		return
	}
	o.baseObject = unstObj
	o.baseYAML = string(d)
}

var script = `
#! /bin/bash
oldFile=%s
oldFileLabel=%s
newFile=%s
newFileLabel=%s

type colordiff &>/dev/null && {
	colordiff -ruwN --color-term-output-only=yes ${oldFile} --label old/${oldFileLabel} ${newFile} --label new/${newFileLabel}
	exit 0
}
diff -ruwN ${oldFile} --label old/${oldFileLabel} ${newFile} --label new/${newFileLabel}
`

func (o *diffobj) diff(left, right string) error {
	file, err := ioutil.TempFile("", o.fileName()+".yaml-")
	if err != nil {
		return err
	}
	defer func() {
		file.Close()
		os.Remove(file.Name())
	}()
	_, err = file.Write([]byte(left))
	if err != nil {
		return err
	}

	file2, err := ioutil.TempFile("", o.fileName()+".yaml-")
	if err != nil {
		return err
	}
	defer func() {
		file2.Close()
		os.Remove(file2.Name())
	}()
	_, err = file2.Write([]byte(right))
	if err != nil {
		return err
	}

	if o.namespace != "" {
		fmt.Fprintf(o.output, "# Resource: kubectl get %s -oyaml -n %s %s @ %v\n", o.resource, o.namespace, o.name, time.Now())
	} else {
		fmt.Fprintf(o.output, "# Resource: kubectl get %s -oyaml %s @ %v\n", o.resource, o.name, time.Now())
	}
	formattedScript := fmt.Sprintf(
		script,
		file.Name(),
		o.labelName(),
		file2.Name(),
		o.labelName(),
	)
	cmd := exec.Command("bash", "-c", formattedScript)
	cmd.Stdout = o.output
	cmd.Stderr = o.output
	_ = cmd.Run()
	return nil
}
