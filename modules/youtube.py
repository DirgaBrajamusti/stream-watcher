import json
import toml
import re
import requests
import subprocess
import threading
from datetime import datetime
import os

config = toml.load('./config.toml')
jobs = []


def get_channel_live(channel_id):
    req  = requests.get(f"https://www.youtube.com/channel/{channel_id}/live")

    yt_lang = re.search('lang="(.*?)"', req.text)[1]
    checker_lang = {'id-ID': "sedang menonton", 'en': "watching now"}
    
    if req.status_code == 200 and '"isLive":true' and checker_lang[yt_lang] in req.text:
        video_id = re.search('(?<=watch\?v=)[0-9A-Za-z-_]{11}', req.text)[0]
        video_title = re.search('<meta name="title" content="(.*?)">', req.text)[1]
        return [{'title': video_title, 'video_id': video_id}]
    else:
        return None

def check_video_regex(channel, videos):
    for video in videos:
        if re.search(channel['filters'][0], video['title']) != None:
            if video['video_id'] not in jobs:
                jobs.append(video['video_id'])
                download_video(channel, video)            
        else:
            print("[youtube] stream is not in filters")

def download_video(channel, video):
    print(f"[youtube] download videos {video['video_id']}")
    command = config['yt-dlp']['executable_path'] + " " + " ".join(config['yt-dlp']['args']) + " " + f"https://www.youtube.com/watch?v={video['video_id']}" + f" -P {config['yt-dlp']['working_directory']}"
    with open(f"./logs/err_{datetime.now().strftime('%Y-%m-%d')} {channel['name']}_{video['video_id']}.txt", 'a') as log_file:
      process = subprocess.Popen(command, shell=True, stdout=subprocess.DEVNULL, stderr=log_file)
    print(f"[subprocess] Downloading {video['video_id']} stream")
    discord_notification(channel, video, "Recording")

    # Wait for the process to finish in a separate thread
    thread = threading.Thread(target=wait_for_download_video, args=(process, channel, video))
    thread.start()

def wait_for_download_video(process, channel, video):
    return_code = process.wait()
    jobs.remove(video['video_id'])
    print(f"[subprocess] Downloading {video['video_id']} done | {return_code}")
    if return_code == 0:
        os.remove(f"./logs/err_{datetime.now().strftime('%Y-%m-%d')} {channel['name']}_{video['video_id']}.txt")
        discord_notification(channel, video, "Done")
    else:
        discord_notification(channel, video, "Error")

def check_channel():
    channels = config['youtube_channel']
    for channel in channels:
        print(f"[youtube] Checking {channel['name']}")
        # Using check live
        videos = get_channel_live(channel['id'])
        if videos is not None:
            print(f'[youtube] {channel["name"]} is live!')
            check_video_regex(channel, videos)

def discord_notification(channel, video, status):
   if config['discord']['notify']:
      url = config['discord']['webhook']
      headers = {'Content-Type': 'application/json'}

      color = {"Recording": 65280, "Done": 9934835, "Error": 16711680}
      payload = json.dumps(
        {
          "content": None,
          "embeds": [
            {
              "title": status,
              "description": video['title'],
              "color": color[status],
              "author": {
                "name": f"{channel['name']} is live!",
                "url": f"https://www.youtube.com/watch?v={video['video_id']}",
                "icon_url": None
              },
              "footer": {
                "text": "Shiodome v0.0.1"
              },
              "thumbnail": {
                "url": f"https://img.youtube.com/vi/{video['video_id']}/0.jpg"
              }
            }
          ],
          "username": "Shiodome",
          "attachments": []
        }
      )
      
      req = requests.post(url, data=payload, headers=headers)
      if req.status_code != 400:
        print("[discord] webhook message send")
      else:
        print("[discord] webhook errored")      