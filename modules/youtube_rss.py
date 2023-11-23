import toml
import re
import requests
import subprocess
import threading
import xml.etree.ElementTree as ET
from datetime import datetime, timedelta

config = toml.load('./config.toml')
youtube_rss = "https://www.youtube.com/feeds/videos.xml?channel_id="

youtube = config['youtube_channel'][0]

jobs = []

def get_videos(xml_data_from_youtube):
    current_time = datetime.utcnow()
    check_time = current_time - timedelta(hours=24)

    content = ET.fromstring(xml_data_from_youtube)
    videos = []
    for entry in content.findall('.//{http://www.w3.org/2005/Atom}entry'):
        published_str = entry.find('{http://www.w3.org/2005/Atom}published').text
        published_time = datetime.strptime(published_str, "%Y-%m-%dT%H:%M:%S%z")
        published_time = published_time.replace(tzinfo=None)

        if published_time > check_time:
            title = entry.find('.//{http://www.w3.org/2005/Atom}title').text
            video_id = entry.find('.//{http://www.youtube.com/xml/schemas/2015}videoId').text
            videos.append({'title': title, 'video_id': video_id})
    return videos

def check_video_regex(channel, videos):
    for video in videos:
        if re.search(channel['filters'][0], video['title']) != None:
            if video['video_id'] not in jobs:
                jobs.append(video['video_id'])
                download_video(video['video_id'])
                

def download_video(video_id):
    print(f"[youtube] download videos {video_id}")
    command = config['yt-dlp']['executable_path'] + " " + " ".join(config['yt-dlp']['args']) + " " + f"https://www.youtube.com/watch?v={video_id}"
    with open(f"./logs/err_{datetime.now().strftime('%Y-%m-%d')} {video_id}.txt", 'a') as log_file:
      process = subprocess.Popen(command, shell=True, stdout=subprocess.DEVNULL, stderr=log_file)
    print(f"[subprocess] Downloading {video_id} stream")

    # Wait for the process to finish in a separate thread
    thread = threading.Thread(target=wait_for_download_video, args=(process, video_id))
    thread.start()

def wait_for_download_video(process, video_id):
    return_code = process.wait()
    jobs.remove(video_id)
    print(f"[subprocess] Downloading {video_id} done | {return_code}")

def check_channel():
    channels = config['youtube_channel']
    for channel in channels:
        print(f"[youtube] Checking {channel['name']}")
        req = requests.get(youtube_rss + channel['id'])
        if req.status_code == 200:
            videos = get_videos(req.text)
            check_video_regex(channel, videos)

check_channel()