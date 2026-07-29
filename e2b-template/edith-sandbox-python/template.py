from e2b import Template

template = (
    Template()
    .from_image("e2bdev/base")
    .make_dir(
        [
            "/home/user/uploads",
            "/home/user/work",
            "/home/user/artifacts",
        ],
        user="user",
    )
)
