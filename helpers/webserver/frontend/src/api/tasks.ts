import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { TaskWithStatus } from '../bindings/TaskWithStatus';
import { rejectError } from './api';
import { CreateTaskRequest } from '../bindings/CreateTaskRequest';

export const stateString = (state: string) => state;

const stateSort = [
  'Recording',
  'Muxing',
  'Waiting',
  'Finished',
  'Idle',
  'Ended',
  'AlreadyProcessed',
  'Interrupted',
  'Error',
];

export const useQueryTasks = () =>
  useQuery(
    ['tasks'],
    () =>
      fetch('/api/tasks')
        .then((res) => res.json())
        .then((res) =>
          (res as TaskWithStatus[]).sort(
            (a, b) =>
              stateSort.indexOf(a.status.state) - stateSort.indexOf(b.status.state)
          )
        ),
    {
      refetchInterval: 1000,
      keepPreviousData: true,
    }
  );

export const useMutateCreateTask = () => {
  const queryClient = useQueryClient();
  return useMutation(
    (task: CreateTaskRequest) =>
      fetch('/api/task', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(task),
      }).then(rejectError),
    {
      onSuccess: () => {
        queryClient.invalidateQueries(['tasks']);
      },
    }
  );
};
