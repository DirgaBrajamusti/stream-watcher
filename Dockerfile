FROM python:3.11.6-alpine3.17

WORKDIR /app

RUN apk update && apk add --no-cache curl bash tzdata ffmpeg yt-dlp

COPY . .
RUN pip install --no-cache-dir -r requirements.txt

CMD [ "python", "-u", "./main.py" ]