import {
  Button,
  HStack,
  Progress,
  ProgressIndicator,
  ProgressLabel,
  Td,
  Tr,
} from "@hope-ui/solid"
import { Show } from "solid-js"
import { useFetch, useRouter, useT } from "~/hooks"
import { MountDetails, PEmptyResp, Storage } from "~/types"
import {
  handleResp,
  handleRespWithNotifySuccess,
  notify,
  r,
  usedPercentage,
  toReadableUsage,
  nearlyFull,
} from "~/utils"
import { DeletePopover } from "../common/DeletePopover"

interface StorageProps {
  storage: Storage
  refresh: () => void
}

function StorageOp(props: StorageProps) {
  const t = useT()
  const { to } = useRouter()
  const [deleteLoading, deleteStorage] = useFetch(
    (): PEmptyResp => r.post(`/admin/storage/delete?id=${props.storage.id}`),
  )
  const [enableOrDisableLoading, enableOrDisable] = useFetch(
    (): PEmptyResp =>
      r.post(
        `/admin/storage/${props.storage.disabled ? "enable" : "disable"}?id=${
          props.storage.id
        }`,
      ),
  )
  return (
    <>
      <Button
        onClick={() => {
          to(`/@manage/storages/edit/${props.storage.id}`)
        }}
      >
        {t("global.edit")}
      </Button>
      <Button
        loading={enableOrDisableLoading()}
        colorScheme={props.storage.disabled ? "success" : "warning"}
        onClick={async () => {
          const resp = await enableOrDisable()
          handleRespWithNotifySuccess(resp, () => {
            props.refresh()
          })
        }}
      >
        {t(`global.${props.storage.disabled ? "enable" : "disable"}`)}
      </Button>
      <DeletePopover
        name={props.storage.mount_path}
        loading={deleteLoading()}
        onClick={async () => {
          const resp = await deleteStorage()
          handleResp(resp, () => {
            notify.success(t("global.delete_success"))
            props.refresh()
          })
        }}
      />
    </>
  )
}

function StorageUsage(props: { details: MountDetails | undefined }) {
  return (
    <Show when={props.details}>
      <Progress
        class="disk-usage-percentage"
        trackColor="$info3"
        rounded="$full"
        size="md"
        value={usedPercentage(props.details!)}
      >
        <ProgressIndicator
          color={nearlyFull(props.details!) ? "$danger6" : "$info6"}
          rounded="$md"
        />
        <ProgressLabel class="disk-usage-text">
          {toReadableUsage(props.details!)}
        </ProgressLabel>
      </Progress>
    </Show>
  )
}

export function StorageListItem(props: StorageProps) {
  const t = useT()
  return (
    <Tr>
      <Td>{props.storage.mount_path}</Td>
      <Td>{props.storage.order}</Td>
      <Td>
        <StorageUsage details={props.storage.mount_details} />
      </Td>
      <Td>
        {t(
          `storages.table_fields.status.${props.storage.status}`,
          undefined,
          props.storage.status,
        )}
      </Td>
      <Td>{props.storage.remark}</Td>
      <Td>
        <HStack spacing="$2">
          <StorageOp {...props} />
        </HStack>
      </Td>
    </Tr>
  )
}
