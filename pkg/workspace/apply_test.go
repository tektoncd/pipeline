package workspace_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/workspace"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

func TestCreateVolumes(t *testing.T) {
	names.TestingSeed()
	for _, tc := range []struct {
		name            string
		workspaces      []v1beta1.WorkspaceBinding
		expectedVolumes map[string]corev1.Volume
	}{{
		name: "binding a single workspace with a PVC",
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/foo/bar/baz",
		}},
		expectedVolumes: map[string]corev1.Volume{
			"custom": {
				Name: "ws-9l9zj",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			},
		},
	}, {
		name: "binding a single workspace with emptyDir",
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "custom",
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
			SubPath: "/foo/bar/baz",
		}},
		expectedVolumes: map[string]corev1.Volume{
			"custom": {
				Name: "ws-mz4c7",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			},
		},
	}, {
		name: "binding a single workspace with configMap",
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "custom",
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "foobarconfigmap",
				},
				Items: []corev1.KeyToPath{{
					Key:  "foobar",
					Path: "foobar.txt",
				}},
			},
			SubPath: "/foo/bar/baz",
		}},
		expectedVolumes: map[string]corev1.Volume{
			"custom": {
				Name: "ws-mssqb",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "foobarconfigmap",
						},
						Items: []corev1.KeyToPath{{
							Key:  "foobar",
							Path: "foobar.txt",
						}},
					},
				},
			},
		},
	}, {
		name: "binding a single workspace with secret",
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "custom",
			Secret: &corev1.SecretVolumeSource{
				SecretName: "foobarsecret",
				Items: []corev1.KeyToPath{{
					Key:  "foobar",
					Path: "foobar.txt",
				}},
			},
			SubPath: "/foo/bar/baz",
		}},
		expectedVolumes: map[string]corev1.Volume{
			"custom": {
				Name: "ws-78c5n",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "foobarsecret",
						Items: []corev1.KeyToPath{{
							Key:  "foobar",
							Path: "foobar.txt",
						}},
					},
				},
			},
		},
	}, {
		name:            "0 workspace bindings",
		workspaces:      []v1beta1.WorkspaceBinding{},
		expectedVolumes: map[string]corev1.Volume{},
	}, {
		name: "binding multiple workspaces",
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/foo/bar/baz",
		}, {
			Name: "even-more-custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "myotherpvc",
			},
		}},
		expectedVolumes: map[string]corev1.Volume{
			"custom": {
				Name: "ws-6nl7g",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			},
			"even-more-custom": {
				Name: "ws-j2tds",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "myotherpvc",
					},
				},
			},
		},
	}, {
		name: "multiple workspaces binding to the same volume with diff subpaths doesnt duplicate",
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/foo/bar/baz",
		}, {
			Name: "custom2",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/very/professional/work/space",
		}},
		expectedVolumes: map[string]corev1.Volume{
			"custom": {
				Name: "ws-vr6ds",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			},
			"custom2": {
				// Since it is the same PVC source, it can't be added twice with two different names
				Name: "ws-vr6ds",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			v := workspace.CreateVolumes(tc.workspaces)
			if d := cmp.Diff(tc.expectedVolumes, v); d != "" {
				t.Errorf("Didn't get expected volumes from bindings %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApply(t *testing.T) {
	names.TestingSeed()
	for _, tc := range []struct {
		name             string
		ts               v1beta1.TaskSpec
		workspaces       []v1beta1.WorkspaceBinding
		expectedTaskSpec v1beta1.TaskSpec
	}{{
		name: "binding a single workspace with a PVC",
		ts: v1beta1.TaskSpec{
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "custom",
			}},
		},
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/foo/bar/baz",
		}},
		expectedTaskSpec: v1beta1.TaskSpec{
			StepTemplate: &v1beta1.StepTemplate{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-9l9zj",
					MountPath: "/workspace/custom",
					SubPath:   "/foo/bar/baz",
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "ws-9l9zj",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "custom",
			}},
		},
	}, {
		name: "binding a single workspace with emptyDir",
		ts: v1beta1.TaskSpec{
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "custom",
			},
			}},
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "custom",
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
			SubPath: "/foo/bar/baz",
		}},
		expectedTaskSpec: v1beta1.TaskSpec{
			StepTemplate: &v1beta1.StepTemplate{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-mz4c7",
					MountPath: "/workspace/custom",
					SubPath:   "/foo/bar/baz", // TODO: what happens when you use subPath with emptyDir
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "ws-mz4c7",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "custom",
			}},
		},
	}, {
		name: "task spec already has volumes and stepTemplate",
		ts: v1beta1.TaskSpec{
			StepTemplate: &v1beta1.StepTemplate{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "awesome-volume",
					MountPath: "/",
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "awesome-volume",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "custom",
			}},
		},
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "custom",
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
			SubPath: "/foo/bar/baz",
		}},
		expectedTaskSpec: v1beta1.TaskSpec{
			StepTemplate: &v1beta1.StepTemplate{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "awesome-volume",
					MountPath: "/",
				}, {
					Name:      "ws-mssqb",
					MountPath: "/workspace/custom",
					SubPath:   "/foo/bar/baz", // TODO: what happens when you use subPath with emptyDir
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "awesome-volume",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}, {
				Name: "ws-mssqb",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "custom",
			}},
		},
	}, {
		name: "0 workspace bindings",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Name: "foo",
			}},
		},
		workspaces: []v1beta1.WorkspaceBinding{},
		expectedTaskSpec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Name: "foo",
			}},
		},
	}, {
		name: "binding multiple workspaces",
		ts: v1beta1.TaskSpec{
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "custom",
			}, {
				Name: "even-more-custom",
			}},
		},
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/foo/bar/baz",
		}, {
			Name: "even-more-custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "myotherpvc",
			},
		}},
		expectedTaskSpec: v1beta1.TaskSpec{
			StepTemplate: &v1beta1.StepTemplate{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-78c5n",
					MountPath: "/workspace/custom",
					SubPath:   "/foo/bar/baz",
				}, {
					Name:      "ws-6nl7g",
					MountPath: "/workspace/even-more-custom",
					SubPath:   "",
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "ws-78c5n",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			}, {
				Name: "ws-6nl7g",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "myotherpvc",
					},
				},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "custom"}, {
				Name: "even-more-custom",
			}},
		},
	}, {
		name: "multiple workspaces binding to the same volume with diff subpaths doesnt duplicate",
		ts: v1beta1.TaskSpec{
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "custom",
			}, {
				Name: "custom2",
			}},
		},
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/foo/bar/baz",
		}, {
			Name: "custom2",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/very/professional/work/space",
		}},
		expectedTaskSpec: v1beta1.TaskSpec{
			StepTemplate: &v1beta1.StepTemplate{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-j2tds",
					MountPath: "/workspace/custom",
					SubPath:   "/foo/bar/baz",
				}, {
					Name:      "ws-j2tds",
					MountPath: "/workspace/custom2",
					SubPath:   "/very/professional/work/space",
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "ws-j2tds",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "custom",
			}, {
				Name: "custom2",
			}},
		},
	}, {
		name: "non default mount path",
		ts: v1beta1.TaskSpec{
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:      "custom",
				MountPath: "/my/fancy/mount/path",
			}},
		},
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
		}},
		expectedTaskSpec: v1beta1.TaskSpec{
			StepTemplate: &v1beta1.StepTemplate{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-l22wn",
					MountPath: "/my/fancy/mount/path",
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "ws-l22wn",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:      "custom",
				MountPath: "/my/fancy/mount/path",
			}},
		},
	}, {
		name: "readOnly true marks volume mount readOnly",
		ts: v1beta1.TaskSpec{
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:      "custom",
				MountPath: "/my/fancy/mount/path",
				ReadOnly:  true,
			}},
		},
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
		}},
		expectedTaskSpec: v1beta1.TaskSpec{
			StepTemplate: &v1beta1.StepTemplate{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-twkr2",
					MountPath: "/my/fancy/mount/path",
					ReadOnly:  true,
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "ws-twkr2",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:      "custom",
				MountPath: "/my/fancy/mount/path",
				ReadOnly:  true,
			}},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			vols := workspace.CreateVolumes(tc.workspaces)
			ts, err := workspace.Apply(context.Background(), tc.ts, tc.workspaces, vols)
			if err != nil {
				t.Fatalf("Did not expect error but got %v", err)
			}
			if d := cmp.Diff(tc.expectedTaskSpec, *ts); d != "" {
				t.Errorf("Didn't get expected TaskSpec modifications %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApply_IsolatedWorkspaces(t *testing.T) {
	names.TestingSeed()
	for _, tc := range []struct {
		name             string
		ts               v1beta1.TaskSpec
		workspaces       []v1beta1.WorkspaceBinding
		expectedTaskSpec v1beta1.TaskSpec
	}{{
		name: "workspace isolated to step does not appear in step template or sidecars",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name: "source",
				}},
			}},
			Sidecars: []v1beta1.Sidecar{{Name: "foo"}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "source",
			}},
		},
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "source",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "testpvc",
			},
		}},
		expectedTaskSpec: v1beta1.TaskSpec{
			StepTemplate: &v1beta1.StepTemplate{},
			Volumes: []corev1.Volume{{
				Name: "ws-9l9zj",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "testpvc",
					},
				},
			}},
			Steps: []v1beta1.Step{{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-9l9zj",
					MountPath: "/workspace/source",
				}},
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name: "source",
				}},
			}},
			Sidecars: []v1beta1.Sidecar{{Name: "foo"}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "source",
			}},
		},
	}, {
		name: "workspace isolated to sidecar does not appear in steps",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Name: "step1",
			}},
			Sidecars: []v1beta1.Sidecar{{
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name: "source",
				}},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "source",
			}},
		},
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "source",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "testpvc",
			},
		}},
		expectedTaskSpec: v1beta1.TaskSpec{
			StepTemplate: &v1beta1.StepTemplate{},
			Volumes: []corev1.Volume{{
				Name: "ws-mz4c7",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "testpvc",
					},
				},
			}},
			Steps: []v1beta1.Step{{
				Name: "step1",
			}},
			Sidecars: []v1beta1.Sidecar{{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-mz4c7",
					MountPath: "/workspace/source",
				}},
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name: "source",
				}},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "source",
			}},
		},
	}, {
		name: "workspace isolated to one step and one sidecar does not appear in step template",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name: "source",
				}},
			}, {
				Name: "step2",
			}},
			Sidecars: []v1beta1.Sidecar{{
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name: "source",
				}},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "source",
			}},
		},
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "source",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "testpvc",
			},
		}},
		expectedTaskSpec: v1beta1.TaskSpec{
			StepTemplate: &v1beta1.StepTemplate{},
			Volumes: []corev1.Volume{{
				Name: "ws-mssqb",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "testpvc",
					},
				},
			}},
			Steps: []v1beta1.Step{{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-mssqb",
					MountPath: "/workspace/source",
				}},
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name: "source",
				}},
			}, {
				Name: "step2",
			}},
			Sidecars: []v1beta1.Sidecar{{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-mssqb",
					MountPath: "/workspace/source",
				}},
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name: "source",
				}},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "source",
			}},
		},
	}, {
		name: "workspaces are mounted to sidecars by default",
		ts: v1beta1.TaskSpec{
			Steps:    []v1beta1.Step{{}},
			Sidecars: []v1beta1.Sidecar{{}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "source",
			}},
		},
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "source",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "testpvc",
			},
		}},
		expectedTaskSpec: v1beta1.TaskSpec{
			StepTemplate: &v1beta1.StepTemplate{
				VolumeMounts: []corev1.VolumeMount{{
					Name: "ws-78c5n", MountPath: "/workspace/source",
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "ws-78c5n",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "testpvc",
					},
				},
			}},
			Steps: []v1beta1.Step{{}},
			Sidecars: []v1beta1.Sidecar{{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-78c5n",
					MountPath: "/workspace/source",
				}},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "source",
			}},
		},
	}, {
		name: "isolated workspaces custom mountpaths appear in volumemounts",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name:      "source",
					MountPath: "/foo",
				}},
			}},
			Sidecars: []v1beta1.Sidecar{{
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name:      "source",
					MountPath: "/bar",
				}},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "source",
			}},
		},
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "source",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "testpvc",
			},
		}},
		expectedTaskSpec: v1beta1.TaskSpec{
			StepTemplate: &v1beta1.StepTemplate{},
			Volumes: []corev1.Volume{{
				Name: "ws-6nl7g",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "testpvc",
					},
				},
			}},
			Steps: []v1beta1.Step{{
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name:      "source",
					MountPath: "/foo",
				}},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-6nl7g",
					MountPath: "/foo",
				}},
			}},
			Sidecars: []v1beta1.Sidecar{{
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name:      "source",
					MountPath: "/bar",
				}},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-6nl7g",
					MountPath: "/bar",
				}},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "source",
			}},
		},
	}, {
		name: "existing sidecar volumeMounts are not displaced by workspace binding",
		ts: v1beta1.TaskSpec{
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:      "custom",
				MountPath: "/my/fancy/mount/path",
				ReadOnly:  true,
			}},
			Sidecars: []v1beta1.Sidecar{{
				Name: "conflicting volume mount sidecar",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "mount-path-conflicts",
					MountPath: "/my/fancy/mount/path",
				}},
			}},
		},
		workspaces: []v1beta1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
		}},
		expectedTaskSpec: v1beta1.TaskSpec{
			StepTemplate: &v1beta1.StepTemplate{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-j2tds",
					MountPath: "/my/fancy/mount/path",
					ReadOnly:  true,
				}},
			},
			Sidecars: []v1beta1.Sidecar{{
				Name: "conflicting volume mount sidecar",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "mount-path-conflicts",
					MountPath: "/my/fancy/mount/path",
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "ws-j2tds",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:      "custom",
				MountPath: "/my/fancy/mount/path",
				ReadOnly:  true,
			}},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := config.ToContext(context.Background(), &config.Config{
				FeatureFlags: &config.FeatureFlags{
					EnableAPIFields: "alpha",
				},
			})
			vols := workspace.CreateVolumes(tc.workspaces)
			ts, err := workspace.Apply(ctx, tc.ts, tc.workspaces, vols)
			if err != nil {
				t.Fatalf("Did not expect error but got %v", err)
			}
			if d := cmp.Diff(tc.expectedTaskSpec, *ts); d != "" {
				t.Errorf("Didn't get expected TaskSpec modifications %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyWithMissingWorkspaceDeclaration(t *testing.T) {
	names.TestingSeed()
	ts := v1beta1.TaskSpec{
		Steps:      []v1beta1.Step{{}},
		Sidecars:   []v1beta1.Sidecar{{}},
		Workspaces: []v1beta1.WorkspaceDeclaration{}, // Intentionally missing workspace declaration
	}
	bindings := []v1beta1.WorkspaceBinding{{
		Name: "source",
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: "testpvc",
		},
	}}
	vols := workspace.CreateVolumes(bindings)
	if _, err := workspace.Apply(context.Background(), ts, bindings, vols); err == nil {
		t.Errorf("Expected error because workspace doesnt exist.")
	}
}

// TestAddSidecarVolumeMount tests that sidecars dont receive a volume mount if
// it has a mount that already shares the same MountPath.
func TestAddSidecarVolumeMount(t *testing.T) {
	for _, tc := range []struct {
		sidecarMounts   []corev1.VolumeMount
		volumeMount     corev1.VolumeMount
		expectedSidecar v1beta1.Sidecar
	}{{
		sidecarMounts: nil,
		volumeMount: corev1.VolumeMount{
			Name:      "foo",
			MountPath: "/workspace/foo",
		},
		expectedSidecar: v1beta1.Sidecar{
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "foo",
				MountPath: "/workspace/foo",
			}},
		},
	}, {
		sidecarMounts: []corev1.VolumeMount{},
		volumeMount: corev1.VolumeMount{
			Name:      "foo",
			MountPath: "/workspace/foo",
		},
		expectedSidecar: v1beta1.Sidecar{
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "foo",
				MountPath: "/workspace/foo",
			}},
		},
	}, {
		sidecarMounts: []corev1.VolumeMount{{
			Name:      "bar",
			MountPath: "/workspace/bar",
		}},
		volumeMount: corev1.VolumeMount{
			Name:      "workspace1",
			MountPath: "/workspace/bar",
		},
		expectedSidecar: v1beta1.Sidecar{
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "bar",
				MountPath: "/workspace/bar",
			}},
		},
	}, {
		sidecarMounts: []corev1.VolumeMount{{
			Name:      "bar",
			MountPath: "/workspace/bar",
		}},
		volumeMount: corev1.VolumeMount{
			Name:      "foo",
			MountPath: "/workspace/foo",
		},
		expectedSidecar: v1beta1.Sidecar{
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "bar",
				MountPath: "/workspace/bar",
			}, {
				Name:      "foo",
				MountPath: "/workspace/foo",
			}},
		},
	}} {
		sidecar := v1beta1.Sidecar{}
		sidecar.VolumeMounts = tc.sidecarMounts
		workspace.AddSidecarVolumeMount(&sidecar, tc.volumeMount)
		if d := cmp.Diff(tc.expectedSidecar, sidecar); d != "" {
			t.Error(diff.PrintWantGot(d))
		}
	}
}
