import {
  Anchor,
  AspectRatio,
  Badge,
  Button,
  Card,
  Container,
  Group,
  Image,
  MediaQuery,
  Select,
  SimpleGrid,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
} from '@mantine/core';
import React from 'react';
import { stateString, useMutateCreateTask, useQueryTasks } from '../api/tasks';
import { TaskWithStatus } from '../bindings/TaskWithStatus';
import { SuspenseLoader } from '../shared/SuspenseLoader';
import { IconPlus, IconExternalLink } from '@tabler/icons';
import { closeAllModals, openModal } from '@mantine/modals';
import { showNotification } from '@mantine/notifications';
import { useQueryConfig } from '../api/config';

const SleepingPanda = React.lazy(() => import('../lotties/SleepingPanda'));

const providerLabel = (provider: string) => {
  switch (provider) {
    case 'ytarchive':
      return 'ytarchive';
    case 'yt-dlp':
      return 'yt-dlp';
    case 'streamlink':
      return 'streamlink';
    default:
      return provider || 'unknown';
  }
};

const providerColor = (provider: string) => {
  switch (provider) {
    case 'ytarchive':
      return 'red';
    case 'yt-dlp':
      return 'blue';
    case 'streamlink':
      return 'violet';
    case 'rplay':
      return 'orange';
    default:
      return 'gray';
  }
};

const platformColor = (platform: string) => {
  switch (platform) {
    case 'youtube':
      return 'red';
    case 'twitch':
      return 'violet';
    case 'rplay':
      return 'orange';
    default:
      return 'gray';
  }
};

const TaskStateBadge = ({ state }: { state: string }) => {
  const color =
    state === 'Recording'
      ? 'green'
      : state === 'Finished'
      ? 'blue'
      : state === 'Muxing'
      ? 'yellow'
      : state === 'Idle' || state === 'AlreadyProcessed' || state === 'Ended'
      ? 'gray'
      : state === 'Interrupted' || state === 'Error'
      ? 'red'
      : 'violet';
  return (
    <Badge color={color} variant="filled">
      {stateString(state)}
    </Badge>
  );
};

const TaskTitle = ({ task }: { task: TaskWithStatus['task'] }) => (
  <>
    <Anchor
      style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}
      href={task.video_url}
      target="_blank"
      rel="noreferrer"
    >
      {task.title}
      <IconExternalLink size={14} />
    </Anchor>
    <Anchor
      color="dimmed"
      style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}
      href={task.channel_url}
      target="_blank"
      rel="noreferrer"
    >
      {task.channel_name}
      <IconExternalLink size={12} />
    </Anchor>
  </>
);

const TaskProgress = ({ status }: { status: TaskWithStatus['status'] }) => {
  if (status.total_size === null || status.total_size === undefined) {
    return <Text color="dimmed">None</Text>;
  }
  return (
    <Text size="sm">
      V: {status.video_fragments || '?'} / A: {status.audio_fragments || '?'}{' '}
      / DL: {status.total_size || '?'}
    </Text>
  );
};

const TaskBadges = ({ task, status }: TaskWithStatus) => (
  <Group spacing="xs">
    <TaskStateBadge state={status.state} />
    <Badge color={platformColor(task.platform)} variant="light">
      {task.platform}
    </Badge>
    <Badge color={providerColor(status.provider)} variant="light">
      {providerLabel(status.provider)}
    </Badge>
  </Group>
);

const DesktopRow = ({ task, status }: TaskWithStatus) => (
  <tr>
    <td>
      <Image width={160} height={90} radius="md" src={task.video_picture} />
    </td>
    <td>
      <Stack spacing={4}>
        <TaskTitle task={task} />
        <TaskBadges task={task} status={status} />
      </Stack>
    </td>
    <td>
      <TaskProgress status={status} />
    </td>
    <td>
      <Text size="xs" color="dimmed" style={{ maxWidth: 300 }} lineClamp={2}>
        {status.last_output}
      </Text>
    </td>
  </tr>
);

const MobileCard = ({ task, status }: TaskWithStatus) => (
  <Card key={task.video_id}>
    <Card.Section>
      <AspectRatio ratio={16 / 9}>
        <Image fit="cover" width="100%" src={task.video_picture} />
      </AspectRatio>
    </Card.Section>
    <Stack my="lg" spacing="md">
      <TaskTitle task={task} />
      <TaskBadges task={task} status={status} />
      <div>
        <Text weight={700}>Progress</Text>
        <TaskProgress status={status} />
      </div>
      {status.last_output && (
        <div>
          <Text weight={700} size="sm">
            Last output
          </Text>
          <Text size="xs" color="dimmed" lineClamp={3}>
            {status.last_output}
          </Text>
        </div>
      )}
    </Stack>
  </Card>
);

const AddVideoModal = () => {
  const qConfig = useQueryConfig();
  const mCreateTask = useMutateCreateTask();

  const [videoURL, setVideoURL] = React.useState('');
  const [destPaths, setDestPaths] = React.useState<
    { value: string; label: string }[]
  >([]);
  const [destPath, setDestPath] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!qConfig.data) return;
    const outPaths = new Set(qConfig.data.channel.map((ch) => ch.outpath));
    if (outPaths.size > destPaths.length) {
      setDestPaths(
        Array.from(outPaths).map((path) => ({ value: path, label: path }))
      );
    }
  }, [qConfig, destPaths]);

  const addVideo = () => {
    if (!destPath || !videoURL) return;

    mCreateTask.mutateAsync(
      {
        video_url: videoURL,
        output_directory: destPath,
      },
      {
        onSuccess() {
          showNotification({
            message: 'Video added',
            color: 'green',
          });
        },
        async onError(err) {
          let message = '';
          if (err instanceof Response) message = await err.text();
          showNotification({
            title: 'Error adding video',
            message,
            color: 'red',
          });
          console.error(err);
        },
      }
    );

    closeAllModals();
  };

  return (
    <Stack spacing="md">
      <TextInput
        label="Video URL"
        placeholder="https://www.youtube.com/watch?v=..."
        data-autofocus
        value={videoURL}
        onChange={(e) => setVideoURL(e.target.value)}
      />
      <Select
        label="Destination Path"
        data={destPaths}
        placeholder="Select a destination path"
        searchable
        creatable
        getCreateLabel={(input) => 'Use ' + input}
        onCreate={(input) => {
          const item = { value: input, label: input };
          setDestPaths((now) => [...now, item]);
          return item;
        }}
        onChange={(e) => setDestPath(e)}
      />
      <Button fullWidth onClick={addVideo}>
        Add
      </Button>
    </Stack>
  );
};

const TasksPage = () => {
  const qTasks = useQueryTasks();
  const tasks = qTasks.data || [];

  const handleAddVideo = () => {
    openModal({
      title: 'Add Video',
      children: <AddVideoModal />,
    });
  };

  if (qTasks.isLoading && !qTasks.data) return <SuspenseLoader />;
  if (tasks.length < 1)
    return (
      <Container size="xs">
        <AspectRatio ratio={1}>
          <React.Suspense fallback={<div />}>
            <SleepingPanda />
          </React.Suspense>
        </AspectRatio>
        <Title>Crickets...</Title>
        <Text>
          There's nothing here yet. Maybe add some more channels to spice things
          up!
        </Text>
        <Group py="md">
          <Button leftIcon={<IconPlus size={18} />} onClick={handleAddVideo}>
            Add video
          </Button>
        </Group>
      </Container>
    );

  return (
    <Stack p="md">
      <Group>
        <Button leftIcon={<IconPlus size={18} />} onClick={handleAddVideo}>
          Add video
        </Button>
      </Group>
      <MediaQuery smallerThan="md" styles={{ display: 'none' }}>
        <Table>
          <thead>
            <tr>
              <th>Thumbnail</th>
              <th>Video</th>
              <th>Progress</th>
              <th>Last output</th>
            </tr>
          </thead>
          <tbody>
            {tasks.map((task) => (
              <DesktopRow key={task.task.video_id} {...task} />
            ))}
          </tbody>
        </Table>
      </MediaQuery>
      <MediaQuery largerThan="md" styles={{ display: 'none' }}>
        <SimpleGrid
          spacing="md"
          cols={2}
          breakpoints={[
            { maxWidth: 'md', cols: 3, spacing: 'md' },
            { maxWidth: 'sm', cols: 2, spacing: 'sm' },
            { maxWidth: 'xs', cols: 1, spacing: 'sm' },
          ]}
        >
          {tasks.map((task) => (
            <MobileCard key={task.task.video_id} {...task} />
          ))}
        </SimpleGrid>
      </MediaQuery>
    </Stack>
  );
};

export default TasksPage;
