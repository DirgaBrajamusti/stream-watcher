# Stream watcher

## Description
> This is my bad script to monitor and download twitch, youtube streams using yt-dlp

## Requirements
- [Python 3.11.4 or newer](https://www.python.org/)
- [ffmpeg](https://www.ffmpeg.org/)
- [yt-dlp](https://github.com/yt-dlp/yt-dlp)

## Installation
```
pip install -r requirements.txt
```

## Usage
1. Make sure you already have `yt-dlp` and `ffmpeg`, you can check using:
```
yt-dlp --version
```

```
ffmpeg -version
```
2. Modify and rename [config_example.toml] to config.toml
3. Start using python
```
python ./main.py
```