import os

from dotenv import load_dotenv
from e2b import Template, default_build_logger
from template import template

load_dotenv()


if __name__ == "__main__":
    if not os.getenv("E2B_API_KEY"):
        raise RuntimeError("E2B_API_KEY is required in .env")

    Template.build(
        template,
        "edith-v0-1-3",
        on_build_logs=default_build_logger(),
    )
