# Makefile for building the tests for Safe Harbor Server.

SHHOST=52.41.169.7
SHPORT=6000

PRODUCTNAME=Utilities
ORG=Scaled Markets
PACKAGENAME=utilities

.DELETE_ON_ERROR:
.ONESHELL:
.SUFFIXES:
.DEFAULT_GOAL: all

SHELL = /bin/sh

CURDIR=$(shell pwd)

.PHONY: all compile clean info
.DEFAULT: all

src_dir = $(CURDIR)/src

build_dir = $(CURDIR)/bin

all: compile

$(build_dir):
	mkdir $(build_dir)

compile: $(build_dir)/$(PACKAGENAME)

$(build_dir)/$(PACKAGENAME): $(build_dir)
	@GOPATH=$(CURDIR) go install $(PACKAGENAME)
