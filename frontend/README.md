# Latency Dashboard Frontend

This is the frontend for the Latency Dashboard, built with React, TypeScript, and Ant Design. It provides a real-time view of latency metrics from your backend services.

## Features

- Real-time updates via WebSocket
- Sortable metrics table
- Expandable rows for detailed metadata
- Responsive design
- Connection status indicator
- Toggle between combined and split views

## Prerequisites

- Node.js (v14 or later)
- npm (v6 or later) or yarn
- Backend server running (see main README for setup)

## Getting Started

1. Install dependencies:
   ```bash
   npm install
   # or
   yarn install
   ```

2. Start the development server:
   ```bash
   npm start
   # or
   yarn start
   ```

3. Open [http://localhost:3000](http://localhost:3000) to view it in your browser.

## Available Scripts

- `npm start` - Runs the app in development mode
- `npm test` - Launches the test runner
- `npm run build` - Builds the app for production
- `npm run eject` - Ejects from Create React App

## Environment Variables

You can configure the following environment variables:

- `REACT_APP_WS_URL` - WebSocket server URL (default: `ws://localhost:8080/ws`)
- `REACT_APP_API_URL` - Backend API URL (default: `http://localhost:8080`)

## Project Structure

```
src/
  components/    # Reusable UI components
  hooks/         # Custom React hooks
  types/         # TypeScript type definitions
  utils/         # Utility functions
  App.tsx        # Main application component
  index.tsx      # Application entry point
  App.css        # Global styles
```

## Development

### Adding Dependencies

Add dependencies using npm or yarn:

```bash
npm install package-name
# or
yarn add package-name
```

### Styling

This project uses CSS Modules for component-scoped styling. Global styles should be added to `App.css`.

## Deployment

To create a production build:

```bash
npm run build
# or
yarn build
```

This will create an optimized production build in the `build` directory.

## License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.
