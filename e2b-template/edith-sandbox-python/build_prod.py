import os

from dotenv import load_dotenv
from e2b import Template, default_build_logger
from template import template

load_dotenv()


if __name__ == "__main__":
    if not os.getenv("E2B_API_KEY"):
        raise RuntimeError("E2B_API_KEY is required in .env")

    template_name = os.getenv("EDITH_E2B_TEMPLATE", "edith-v0-1-4")
    Template.build(
        template,
        template_name,
        on_build_logs=default_build_logger(),
    )
