package serialize

import (
	"ociregistry/impl/upstream"
	"os"
	"path/filepath"
	"testing"
)

func Test1(t *testing.T) {
	td, _ := os.MkdirTemp("", "")
	digest := "406945b5115423a8c1d1e5cd53222ef2ff0ce9d279ed85badbc4793beebebc6c"
	mh := upstream.ManifestHolder{
		ImageUrl:  "registry.k8s.io/kube-scheduler:v1.29.1",
		MediaType: "application/vnd.docker.distribution.manifest.v2+json",
		Digest:    digest,
		Size:      2043,
		Bytes:     []byte("TEST"),
		Ml:        upstream.ManifestList{},
		Im: upstream.ImageManifest{
			SchemaVersion: 2,
			MediaType:     "application/vnd.docker.distribution.manifest.v2+json",
			Config: upstream.ManifestConfig{
				MediaType: "application/vnd.docker.container.image.v1+json",
				Digest:    "sha256:" + digest,
				Size:      2425,
			},
			Layers: []upstream.ManifestLayer{
				{
					MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
					Size:      84572,
					Digest:    "sha256:aba5379b9c6dc7c095628fe6598183d680b134c7f99748649dddf07ff1422846",
				},
				{
					MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
					Size:      12594,
					Digest:    "sha256:e5dbef90bae3c9df1dfd4ae7048c56226f6209d538c91f987aff4f54e888f566",
				},
				{
					MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
					Size:      452856,
					Digest:    "sha256:fbe9343cb4af98ca5a60b6517bf45a5a4d7f7172fb4793d4b55c950196089cda",
				},
				{
					MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
					Size:      317,
					Digest:    "sha256:fcb6f6d2c9986d9cd6a2ea3cc2936e5fc613e09f1af9042329011e43057f3265",
				},
				{
					MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
					Size:      198,
					Digest:    "sha256:e8c73c638ae9ec5ad70c49df7e484040d889cca6b4a9af056579c3d058ea93f0",
				},
				{
					MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
					Size:      113,
					Digest:    "sha256:1e3d9b7d145208fa8fa3ee1c9612d0adaac7255f1bbc9ddea7e461e0b317805c",
				},
				{
					MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
					Size:      385,
					Digest:    "sha256:4aa0ea1413d37a58615488592a0b827ea4b2e48fa5a77cf707d0e35f025e613f",
				},
				{
					MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
					Size:      343,
					Digest:    "sha256:65efb1cabba44ca8eefa2058ebdc19b7f76bbb48400ff9e32b809be25f0cdefa",
				},
				{
					MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
					Size:      129151,
					Digest:    "sha256:13547472c521121fc04c8fa473757115ef8abe698cc9fa67e828371feeff40e7",
				},
				{
					MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
					Size:      734919,
					Digest:    "sha256:53f492e4d27a1a1326e593efdaffcb5e2b0230dc661b20a81a04fa740a37cb4c",
				},
				{
					MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
					Size:      17099135,
					Digest:    "sha256:6523efc24f16435b7507a67c2a1f21828c9d58531902856b294bf49d04b96bbe",
				},
			},
		},
		Tarfile: "/foo/bar/baz",
		Type:    upstream.ImageManifestType,
	}
	ToFilesystem(mh, td)
	fname := filepath.Join(td, "imgmf", digest)
	_, err := os.Stat(fname)
	if err != nil {
		t.Fail()
	}
}
