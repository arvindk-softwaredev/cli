// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pipeline

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/actions"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/deleter"
	"github.com/tektoncd/cli/pkg/formatted"
	"github.com/tektoncd/cli/pkg/options"
	pipelinepkg "github.com/tektoncd/cli/pkg/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/multierr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// pipelineExists validates that the arguments are valid pipeline names
func pipelineExists(args []string, p cli.Params) ([]string, error) {

	availablePNames := make([]string, 0)
	c, err := p.Clients()
	if err != nil {
		return availablePNames, err
	}
	var errorList error
	ns := p.Namespace()
	for _, name := range args {
		var pipeline *v1.Pipeline
		err := actions.GetV1(pipelineGroupResource, c, name, ns, metav1.GetOptions{}, &pipeline)
		if err != nil {
			errorList = multierr.Append(errorList, err)
			continue
		}
		availablePNames = append(availablePNames, name)
	}
	return availablePNames, errorList
}

func deleteCommand(p cli.Params) *cobra.Command {
	opts := &options.DeleteOptions{Resource: "Pipeline", ForceDelete: false, DeleteRelated: false, DeleteAllNs: false}
	var output string
	eg := `Delete Pipelines with names 'foo' and 'bar' in namespace 'quux'

    tkn pipeline delete foo bar -n quux

or

    tkn p rm foo bar -n quux

Delete a Pipeline and print the result as JSON:

    tkn pipeline delete foo -f -o json

Delete a Pipeline and print the result as YAML:

    tkn pipeline delete foo -f -o yaml
`

	c := &cobra.Command{
		Use:               "delete",
		Aliases:           []string{"rm"},
		Short:             "Delete Pipelines in a namespace",
		Example:           eg,
		Args:              cobra.MinimumNArgs(0),
		ValidArgsFunction: formatted.ParentCompletion,
		SilenceUsage:      true,
		Annotations: map[string]string{
			"commandType": "main",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := formatted.ValidateOutput(output); err != nil {
				return err
			}

			s := &cli.Stream{
				In:  cmd.InOrStdin(),
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			}

			availablePNames, errs := pipelineExists(args, p)
			if len(availablePNames) == 0 && errs != nil {
				return errs
			}

			if err := opts.CheckOptions(s, availablePNames, p.Namespace()); err != nil {
				return err
			}

			if err := deletePipelines(opts, s, p, availablePNames, output); err != nil {
				return err
			}
			return errs
		},
	}
	c.Flags().StringVarP(&output, "output", "o", "", formatted.OutputFlagUsage)
	c.Flags().BoolVarP(&opts.ForceDelete, "force", "f", false, "Whether to force deletion (default: false)")
	c.Flags().BoolVarP(&opts.DeleteRelated, "prs", "", false, "Whether to delete Pipeline(s) and related resources (PipelineRuns) (default: false)")
	c.Flags().BoolVarP(&opts.DeleteAllNs, "all", "", false, "Delete all Pipelines in a namespace (default: false)")

	return c
}

func deletePipelines(opts *options.DeleteOptions, s *cli.Stream, p cli.Params, pNames []string, output string) error {
	pipelinerunGroupResource := schema.GroupVersionResource{Group: "tekton.dev", Resource: "pipelineruns"}

	cs, err := p.Clients()
	if err != nil {
		return fmt.Errorf("failed to create tekton client")
	}
	d := deleter.New("Pipeline", func(pipelineName string) error {
		return actions.Delete(pipelineGroupResource, cs.Dynamic, cs.Tekton.Discovery(), pipelineName, p.Namespace(), metav1.DeleteOptions{})
	})
	switch {
	case opts.DeleteAllNs:
		pNames, err = pipelinepkg.GetAllPipelineNames(pipelineGroupResource, cs, p.Namespace())
		if err != nil {
			return err
		}
		d.Delete(pNames)
	case opts.DeleteRelated:
		d.WithRelated("PipelineRun", pipelineRunLister(cs, p.Namespace()), func(pipelineRunName string) error {
			return actions.Delete(pipelinerunGroupResource, cs.Dynamic, cs.Tekton.Discovery(), pipelineRunName, p.Namespace(), metav1.DeleteOptions{})
		})
		deletedPipelineNames := d.Delete(pNames)
		d.DeleteRelated(deletedPipelineNames)
	default:
		d.Delete(pNames)
	}

	if formatted.IsStructured(output) {
		deleted := d.Deleted()
		if deleted == nil {
			deleted = []string{}
		}
		related := d.RelatedDeleted()
		if related == nil {
			related = []string{}
		}
		result := formatted.DeleteResult{
			Kind:      "Pipeline",
			Deleted:   deleted,
			Namespace: p.Namespace(),
			All:       opts.DeleteAllNs,
		}
		if opts.DeleteRelated {
			result.RelatedKind = "PipelineRun"
			result.RelatedDeleted = related
		}
		if err := formatted.PrintStructuredOutput(s.Out, output, result); err != nil {
			return err
		}
		return d.Errors()
	}

	status := &cli.Stream{Out: s.Err, Err: s.Err}
	if !opts.DeleteAllNs {
		d.PrintSuccesses(status)
	} else if opts.DeleteAllNs {
		if d.Errors() == nil {
			fmt.Fprintf(s.Err, "All Pipelines deleted in namespace %q\n", p.Namespace())
		}
	}
	return d.Errors()
}

func pipelineRunLister(cs *cli.Clients, ns string) func(string) ([]string, error) {
	return func(pipelineName string) ([]string, error) {
		lOpts := metav1.ListOptions{
			LabelSelector: fmt.Sprintf("tekton.dev/pipeline=%s", pipelineName),
		}
		var pipelineRuns *v1.PipelineRunList
		if err := actions.ListV1(pipelineRunGroupResource, cs, lOpts, ns, &pipelineRuns); err != nil {
			return nil, err
		}

		var names []string
		for _, pr := range pipelineRuns.Items {
			names = append(names, pr.Name)
		}
		return names, nil
	}
}
