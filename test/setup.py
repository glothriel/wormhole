#!/usr/bin/env python
from setuptools import find_packages, setup

setup(
    name="wormhole",
    version="1.0.0",
    description="Integration tests for wormhole",
    author="Konstanty Karagiorgis",
    author_email="use.my.github@for.contact",
    packages=find_packages(),
    python_requires=">=3.10.0",
    install_requires=["retry==0.9.2", "pytest==6.2.5", "requests==2.26.00"],
)
