# Hot Reload Middleware for Go Web Frameworks

## Current Status
This is a prototype version. Let me know if you find the project useful and suggest new features/frameworks.

## Motivation
The DX for Go web development is not great. There's no way to hot reload templates or static files on file save which
means that a manual browser refresh is required. The Javascript/Typescript world is much better served with tools like
Webpack which offer a hot reloading dev server. This project aims to bridge the gap.

## Features
* Watches the specified Go html templates for changes and reloads templates on change
* Watches the specified static asset folder for changes and signal that a reloads is required
* Inject a client side script into HTML pages which listens on a web socket and reloads pages on dependency change. The web socket reconnects automatically on load/failure

## Usage
See the example Echo server to get started. In essence:
* Define your templates folder and create your templates
* Define and populate your static assets folder
* [Optional] Build any client-side bundles with Webpack (or whatever). They should build to the static assets folder.
* Add the middleware (just for development, there's no need for this in prod)
* Add the template renderer to your server instance
* Add your framework's static asset middleware

## Example App
The example app demonstrates reloading templates and static files. It includes a Webpack config which rebuilds a client
bundle and saves it to the static asset directory. The bundle includes TailwindCSS (with custom font) and HTMX In order to hot reload changes to the bundle you'll need to run
Webpack in watch mode via:

`pnpm install`

`pnpm run watch`