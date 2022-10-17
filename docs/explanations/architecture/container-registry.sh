#!/bin/bash

#pikchr            container-registry.pikchr > container-registry.html
pikchr --svg-only container-registry.pikchr > container-registry.svg
rsvg-convert -f png container-registry.svg  > container-registry.png
