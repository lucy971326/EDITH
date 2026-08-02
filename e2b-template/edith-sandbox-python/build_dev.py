import os

from dotenv import load_dotenv
from e2b import Template, default_build_logger
from template import template

load_dotenv()


if __name__ == "__main__":
    if not os.getenv("E2B_API_KEY"):
        raise RuntimeError("E2B_API_KEY is required in .env")

    template_name = os.getenv("EDITH_E2B_TEMPLATE", "edith-v0-1-5")
    Template.build(
        template,
        f"{template_name}-dev",
        on_build_logs=default_build_logger(),
    )
