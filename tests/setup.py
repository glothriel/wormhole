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
    install_requires=[
        "cryptography==36.0.2",
        "pylama==8.3.7",
        "pytest==7.0.1",
        "psutil==5.9.0",
        "PyMySQL==1.0.2",
        "requests==2.27.1",
        "retry==0.9.2",
    ],
)
