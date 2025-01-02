# Super Bad Stream Watcher

Super Bad Stream Watcher is a Go-based application designed to monitor and download live streams from YouTube and Twitch but i don't know what i'm doing in go.

## Features

- Monitor YouTube and Twitch channels for live streams.
- Download live streams using `yt-dlp` and `ytarchive`
- Send notifications to Discord when a stream starts or finishes.

## Installation

1. Clone the repository:

```sh
git clone https://github.com/yourusername/super-bad-stream-watcher.git
cd super-bad-stream-watcher
```

2. Install dependencies:

```sh
go mod tidy
```

3. Build the project:

```sh
make build
```

## Configuration

1. Copy the example configuration file:

```sh
cp config.toml.example config.toml
touch cookies.txt
```

2. Edit `config.toml` to configure your settings, including YouTube and Twitch channels, Discord webhook, and download paths.

## Running the Application

1. Start the application:

```sh 
./super-bad-stream-watcher
```

2. The web server will be available at http://localhost:3000.

## Usage

- The application will automatically check the configured YouTube and Twitch channels for live streams based on the interval specified in the configuration.
- When a live stream is detected, it will start downloading the stream and send a notification to the configured Discord webhook.
- You can view and manage the download jobs through the web server.

## Acknowledgements

- [yt-dlp](https://github.com/yt-dlp/yt-dlp)
- [ytarchive](https://github.com/Kethsar/ytarchive)
- [golog](https://github.com/kataras/golog)
- [Vite](https://vitejs.dev/)
- [React](https://reactjs.org/)
- [Hoshinova](https://github.com/HoloArchivists/hoshinova)

---
