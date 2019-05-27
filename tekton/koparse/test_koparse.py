#!/usr/bin/env python3.6

import os
import unittest

import koparse


IMAGE_BASE = "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/"
PATH_TO_TEST_RELEASE_YAML = os.path.join(os.path.dirname(
    os.path.abspath(__file__)), "test_release.yaml")
PATH_TO_WRONG_FILE = os.path.join(os.path.dirname(
    os.path.abspath(__file__)), "koparse.py")
BUILT_IMAGES = [
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/kubeconfigwriter@sha256:68453f5bb4b76c0eab98964754114d4f79d3a50413872520d8919a6786ea2b35",
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/creds-init@sha256:67448da79e4731ab534b91df08da547bc434ab08e41d905858f2244e70290f48",
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/git-init@sha256:7d5520efa2d55e1346c424797988c541327ee52ef810a840b5c6f278a9de934a",
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/nop@sha256:3784d6b8f73043a29d2c1d6196801bee46fe808fbb94ba4fd21ca52dce503183",
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/bash@sha256:d55917ef5c92627027e3755bfc577fbfa2fb783cccfb13a98632cb6ba6088cd6",
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/gsutil@sha256:421a261436e16af4057b4a069fdae8a5aca6e37269952209ad9932a774aa0003",
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/controller@sha256:bdc6f22a44944c829983c30213091b60f490b41f89577e8492f6a2936be0df41",
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/webhook@sha256:cca7069a11aaf0d9d214306d456bc40b2e33e5839429bf07c123ad964d495d8a",
]
EXPECTED_IMAGES = [
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/kubeconfigwriter",
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/creds-init",
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/git-init",
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/nop",
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/bash",
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/gsutil",
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/controller",
    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/webhook",
]


class TestKoparse(unittest.TestCase):

    def test_parse_release(self):
        images = koparse.parse_release(IMAGE_BASE, PATH_TO_TEST_RELEASE_YAML)
        self.assertListEqual(images, BUILT_IMAGES)

    def test_parse_release_no_file(self):
        with self.assertRaises(IOError):
            koparse.parse_release(IMAGE_BASE, "whoops")

    def test_parse_release_wrong_contents(self):
        images = koparse.parse_release(IMAGE_BASE, PATH_TO_WRONG_FILE)
        self.assertEqual(images, [])

    def test_compare_expected_images(self):
        koparse.compare_expected_images(EXPECTED_IMAGES, BUILT_IMAGES)

    def test_compare_expected_images_bad_format(self):
        with self.assertRaises(koparse.BadActualImageFormatError):
            koparse.compare_expected_images(EXPECTED_IMAGES, EXPECTED_IMAGES)

    def test_compare_expected_images_missing(self):
        extra_expected = (EXPECTED_IMAGES[:] +
                          ["gcr.io/knative-releases/something-else"])
        with self.assertRaises(koparse.ImagesMismatchError):
            koparse.compare_expected_images(extra_expected, BUILT_IMAGES)

    def test_compare_expected_images_too_many(self):
        extra_actual = (BUILT_IMAGES[:] +
                        ["gcr.io/knative-releases/something-else@sha256:somedigest"])
        with self.assertRaises(koparse.ImagesMismatchError):
            koparse.compare_expected_images(EXPECTED_IMAGES, extra_actual)


if __name__ == "__main__":
    unittest.main()
