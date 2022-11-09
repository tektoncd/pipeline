# Tekton Pipelines Roadmap

Our community roadmap can be found [here](https://github.com/orgs/tektoncd/projects/26/views/16).
This project automatically includes issues and PRs with label `area/roadmap`.

See [The Tekton mission and vision](https://github.com/tektoncd/community/blob/main/roadmap.md#mission-and-vision)
for more information on our long-term goals.

## 2021 Roadmap

*2021 Roadmap appears below for historic purposes.*

* [v1 API](https://github.com/tektoncd/pipeline/issues/3548)
  * Support alpha fields within v1 and beta types ([TEP-0033](https://github.com/tektoncd/community/blob/main/teps/0033-tekton-feature-gates.md))
* When expressions:
  * Option to skip Task only ([TEP-0007](https://github.com/tektoncd/community/blob/main/teps/0007-conditions-beta.md#skipping-1))
  * [Support in Finally Tasks](https://github.com/tektoncd/pipeline/issues/3438) ([TEP-0045](https://github.com/tektoncd/community/blob/main/teps/0045-whenexpressions-in-finally-tasks.md))
* [Env vars at runtime](https://github.com/tektoncd/pipeline/issues/1606)
* Workspaces/PVCs/Affinity Assistant ([TEP-0046](https://github.com/tektoncd/community/pull/318)
  * [Investigate custom scheduler for PVCs](https://github.com/tektoncd/pipeline/issues/3052)
* [OCI bundles to beta](https://github.com/tektoncd/pipeline/issues/3661) (and then v1)
* Expanded expression support via [CelRun Task](https://github.com/tektoncd/pipeline/issues/3149)
* [Pipelines in Pipelines](https://github.com/tektoncd/pipeline/issues/2134)
* Support for more complex failure scenarios
  * Allow task failure ([TEP-0050](https://github.com/tektoncd/community/pull/342))
  * Allow step failure ([TEP-0040](https://github.com/tektoncd/community/pull/302))
  * Decisions regarding more complex graph construction on failure
* [Instrument Tekton resources](https://github.com/tektoncd/pipeline/issues/2814)
  * Minimize overhead of running a pipeline
* Improve ability to compose Tasks with Tasks ([TEP-0044](https://github.com/tektoncd/community/pull/316)) 
* [Workspaces “from” other Tasks (express resource dependencies on workspaces)](https://github.com/tektoncd/pipeline/issues/3109)
* [Custom Tasks](https://github.com/tektoncd/community/blob/main/teps/0002-custom-tasks.md) completion:
  * [Pipeline Results](https://github.com/tektoncd/pipeline/issues/3595)
  * Example controller for folks who want to create custom tasks
  * Experimental custom tasks promotion (e.g. [CELRun](https://github.com/tektoncd/experimental/tree/main/cel)):
    * Plan around how to promote (what requirements, process)
    * Continuous integration + release automation
    * Documentation and examples at tekton.dev
    * Integration with operator
* [Adding support for other architectures](https://github.com/tektoncd/pipeline/issues/856)
* [Improve UX of getting credentials into Tasks](https://github.com/tektoncd/pipeline/issues/2343) - nearly complete
  via [TEP-0029](https://github.com/tektoncd/community/blob/main/teps/0029-step-workspaces.md)
* [Notifications](https://github.com/tektoncd/pipeline/issues/1740)
* [Performant Tekton](https://github.com/tektoncd/pipeline/issues/540)
  ([TEP-0036](https://github.com/tektoncd/community/blob/main/teps/0036-start-measuring-tekton-pipelines-performance.md))
* [Debug mode](https://github.com/tektoncd/pipeline/issues/2069)
* [PipelineResources: beta or bust](https://github.com/tektoncd/pipeline/issues/1673)
  ([some discussion and analysis](https://docs.google.com/document/d/1Et10YdBXBe3o2x6lCfTindFnuBKOxuUGESLb__t11xk/edit#heading=h.xz4bckr3atww))
* [Looping syntax](https://github.com/tektoncd/pipeline/issues/2050)
* [Concurrency limits](https://github.com/tektoncd/experimental/issues/699)
  ([TEP-0013](https://github.com/tektoncd/community/pull/228))
* [Partial Pipeline execution](https://github.com/tektoncd/pipeline/issues/50)
* [Testing tools](https://github.com/tektoncd/pipeline/issues/1289)
* Decisions around what to do with SCM support and the images released as part of Tekton Pipelines to support
  [PipelineResources](https://github.com/tektoncd/pipeline/issues/1673)
* [Rich type support for Params](https://github.com/tektoncd/pipeline/issues/1393)
* Decide if these are in scope for Tekton Pipelines:
  * [Local execution](https://github.com/tektoncd/pipeline/issues/235)
    (and [tektoncd/community#145](https://github.com/tektoncd/community/issues/145))
  * [Config as code](https://github.com/tektoncd/pipeline/issues/859)
    ([TEP-0048](https://github.com/tektoncd/community/pull/341),
    [task references via git](https://github.com/tektoncd/pipeline/issues/2298))
* [Rework PipelineRun and TaskRun Status](https://github.com/tektoncd/pipeline/issues/3792)
