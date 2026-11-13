#!/bin/bash

# Step 1 - build capa nativa de WeasyPrint (librerias pango/cairo para AL2023)
if [ ! -d layers/weasyprint-al2023/lib ]; then
    echo "Building weasyprint layer"
    bash layers/weasyprint-al2023/build.sh
fi

# Step 2 - build
# Puede usar --use-container
echo "Building app"
sam build --use-container

# Step 3 - Run
echo "Running"
sam local start-api --env-vars env.json

