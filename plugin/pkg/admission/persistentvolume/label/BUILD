package(default_visibility = ["//visibility:public"])

licenses(["notice"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "admission.go",
        "doc.go",
    ],
    tags = ["automanaged"],
    deps = [
        "//pkg/admission:go_default_library",
        "//pkg/api:go_default_library",
        "//pkg/cloudprovider:go_default_library",
        "//pkg/cloudprovider/providers/aws:go_default_library",
        "//pkg/cloudprovider/providers/gce:go_default_library",
        "//pkg/volume:go_default_library",
        "//vendor:k8s.io/apimachinery/pkg/apis/meta/v1",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["admission_test.go"],
    library = ":go_default_library",
    tags = ["automanaged"],
    deps = [
        "//pkg/admission:go_default_library",
        "//pkg/api:go_default_library",
        "//pkg/cloudprovider/providers/aws:go_default_library",
        "//vendor:k8s.io/apimachinery/pkg/types",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
)
