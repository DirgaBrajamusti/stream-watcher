import toml
from modules import twitch, youtube
from apscheduler.schedulers.background import BackgroundScheduler

config = toml.load("./config.toml")

def twitch_watcher():
    twitch.check_user()

def youtube_watcher():
    youtube.check_channel()


if __name__ == '__main__':
    scheduler = BackgroundScheduler()
    
    if config['archive']['twitch'] and 'twitch_channel' in config:
        print('[system] twitch watcher true')
        scheduler.add_job(twitch_watcher, 'interval', minutes=1)
    
    if config['archive']['youtube'] and 'youtube_channel' in config:
        print('[system] youtube watcher true')
        scheduler.add_job(youtube_watcher, 'interval', minutes=1)

    scheduler.start()
    print("[system] started")

    try:
        while True:
            pass
    except (KeyboardInterrupt, SystemExit):
        scheduler.shutdown()
