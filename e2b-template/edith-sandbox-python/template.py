from pathlib import Path

from e2b import Template

PROJECT_ROOT = Path(__file__).resolve().parents[2]
SKILLS_SOURCE = "backend-v2/internal/skills/system"

template = (
    Template(file_context_path=PROJECT_ROOT)
    .from_image("e2bdev/base")
    .make_dir(
        [
            "/home/user/uploads",
            "/home/user/work",
            "/home/user/artifacts",
        ],
        user="user",
    )
    .make_dir(
        [
            "/home/user/skills/system",
            "/home/user/skills/custom",
        ],
        user="root",
    )
    .copy(
        SKILLS_SOURCE,
        "/home/user/skills/system/",
        user="root",
    )
)
