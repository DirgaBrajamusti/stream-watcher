import requests
import toml
import subprocess
import threading
import datetime
import json
import os
import re

config = toml.load("./config.toml")
jobs = []

def check_user():
  for channel in config['twitch_channel']:
    if channel['name'] in jobs:
      pass
    else:   
      channel_info = get_channel_info(channel['name'])
      if channel_info['live']:
        jobs.append(channel['name'])
        if check_video_regex(channel['filters'], channel_info['title']):
          archive_stream(channel['name'], channel_info['title'], channel_info['thumbnail'], channel_info['profile_image'])

def check_video_regex(filter, title):
    if re.search(filter[0], title) != None:
        return True           
    else:
        print("[twitch] stream is not in filters")
        return False

def get_channel_info(username):
  url = "https://gql.twitch.tv/gql"
  query = "query {\n  user(login: \""+username+"\") {\n profileImageURL(width:50) \n    stream {\n      id\n title \n previewImageURL(height: 720, width:1280)    }\n  }\n}"
  print(f"[twitch] Checking {username}")

  req = requests.request("POST", url, json={"query": query, "variables": {}}, headers={"client-id": "kimne78kx3ncx6brgo4mv6wki5h1ko"}).json()
  
  stream_info = req["data"]["user"]["stream"]
  stream_profile = req["data"]["user"]

  if stream_info == None:
    return {"live": False}
  return {"live": True, "title": stream_info['title'], "thumbnail": stream_info['previewImageURL'], "profile_image": stream_profile['profileImageURL']}

def archive_stream(username, title, thumbnail, profile_image):
    command = config['yt-dlp']['executable_path'] + " " + " ".join(config['yt-dlp']['args']) + " " + f"https://twitch.tv/{username}" + f" -P {config['yt-dlp']['working_directory']}"
    with open(f"./logs/err_{datetime.datetime.now().strftime('%Y-%m-%d')} {username}.txt", 'a') as log_file:
      process = subprocess.Popen(command, shell=True, stdout=subprocess.DEVNULL, stderr=log_file)
    discord_notification(username, title, thumbnail, profile_image, "Recording")
    print(f"[subprocess] Downloading {username} stream")

    # Wait for the process to finish in a separate thread
    thread = threading.Thread(target=wait_for_archive, args=(process, username, title, thumbnail, profile_image))
    thread.start()

def wait_for_archive(process, username, title, thumbnail, profile_image):
    return_code = process.wait()
    jobs.remove(username)
    print(f"[subprocess] Downloading {username} done | {return_code}")
    if return_code == 0:
      os.remove(f"./logs/err_{datetime.datetime.now().strftime('%Y-%m-%d')} {username}.txt")
      discord_notification(username, title, thumbnail, profile_image, "Done")
    else:
      discord_notification(username, title, thumbnail, profile_image, "Error")
 

def discord_notification(username, title, thumbnail, profile_image, status):
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
              "description": title,
              "color": color[status],
              "author": {
                "name": f"{username} is live!",
                "url": f"https://twitch.tv/{username}",
                "icon_url": profile_image
              },
              "footer": {
                "text": "Shiodome v0.0.1"
              },
              "thumbnail": {
                "url": thumbnail
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