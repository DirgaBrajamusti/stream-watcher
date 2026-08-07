export interface Task {
  title: string;
  video_id: string;
  video_url: string;
  video_picture: string;
  channel_name: string;
  channel_id: string;
  channel_url: string;
  channel_picture: string | null;
  output_directory: string;
  platform: string;
}
