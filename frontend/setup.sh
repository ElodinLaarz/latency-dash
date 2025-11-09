#!/bin/bash

# Create necessary directories
mkdir -p src/{components,hooks,types,utils,styles}

# Create initial files
touch src/App.tsx src/index.css src/reportWebVitals.ts

# Install dependencies
npm install

echo "Frontend setup complete! Run 'npm start' to start the development server."
