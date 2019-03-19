#!/usr/bin/env python3

"""
koparse.py parses release.yaml files from `ko`

The `ko` tool (https://github.com/google/go-containerregistry/tree/master/cmd/ko)
builds images and embeds the full names of the built images in the resulting
yaml files.

This script does two things:

* Parses those image names out of the release.yaml, including their digests, and
  outputs those to stdout
* Verifies the list of built images against an expected list, to be sure that all
  expected images were built (and no extra images were built)
"""

import argparse
import os
import re
import string
import sys
from typing import List


DIGEST_MARKER = "@sha256"


class ImagesMismatchError(Exception):
    def __init__(self, missing: List[str], extra: List[str]):
        self.missing = missing
        self.extra = extra

    def __str__(self):
        errs = []
        if self.missing:
            errs.append("Images %s were expected but missing." % self.missing)
        if self.extra:
            errs.append("Images %s were present but not expected." %
                        self.extra)
        return " ".join(errs)


class BadActualImageFormatError(Exception):
    def __init__(self, image: str):
        self.image = image

    def __str__(self):
        return "Format of image %s was unexpected, did not contain %s" % (self.image, DIGEST_MARKER)


def parse_release(base: str, path: str) -> List[str]:
    """Extracts built images from the release.yaml at path

    Args:
        base: The built images will be expected to start with this string,
            other images will be ignored
        path: The path to the file (release.yaml) that will contain the built images
    Returns:
        list of the images parsed from the file
    """
    images = []
    with open(path) as f:
        for line in f:
            match = re.search(base + ".*" + DIGEST_MARKER + ":[0-9a-f]*", line)
            if match:
                images.append(match.group(0))
    return images


def compare_expected_images(expected: List[str], actual: List[str]) -> None:
    """Ensures that the list of actual images includes only the expected images

    Args:
        expected: A list of all of the names of images that are expected to have
            been built, including the path to the image without the digest
        actual: A list of the names of the built images, including the path to the
            image and the digest
    """
    for image in actual:
        if DIGEST_MARKER not in image:
            raise BadActualImageFormatError(image)

    actual_no_digest = [image.split(DIGEST_MARKER)[0] for image in actual]

    missing = set(expected) - set(actual_no_digest)
    extra = set(actual_no_digest) - set(expected)

    if missing or extra:
        raise ImagesMismatchError(list(missing), list(extra))


if __name__ == "__main__":
    arg_parser = argparse.ArgumentParser(
        description="Parse expected built images from a release.yaml created by `ko`")
    arg_parser.add_argument("--path", type=str, required=True,
                            help="Path to the release.yaml")
    arg_parser.add_argument("--base", type=str, required=True,
                            help="String prefix which is used to find images within the release.yaml")
    arg_parser.add_argument("--images", type=str, required=True, nargs="+",
                            help="List of all images expected to be built, without digests")
    args = arg_parser.parse_args()

    try:
        images = parse_release(args.base, args.path)
        compare_expected_images(args.images, images)
    except (IOError, BadActualImageFormatError) as e:
        sys.stderr.write("Error determining built images: %s\n" % e)
        sys.exit(1)
    except (ImagesMismatchError) as e:
        sys.stderr.write("Expected images did not match: %s\n" % e)
        with open(args.path) as f:
            sys.stderr.write(f.read())
        sys.exit(1)

    print("\n".join(images))
