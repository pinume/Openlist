import {
  Box,
  Button,
  HStack,
  Table,
  Tbody,
  Text,
  Th,
  Thead,
  Tr,
  VStack,
} from "@hope-ui/solid"
import { createSignal, For, Show } from "solid-js"
import { useFetch, useManageTitle, useRouter, useT } from "~/hooks"
import { handleResp, notify, r } from "~/utils"
import { EmptyResp, PageResp, Storage } from "~/types"
import { StorageListItem } from "./Storage"

const Storages = () => {
  const t = useT()
  useManageTitle("manage.sidemenu.storages")
  const { to } = useRouter()
  const [getStoragesLoading, getStorages] = useFetch(
    (): Promise<PageResp<Storage>> => r.get("/admin/storage/list"),
  )
  const [storages, setStorages] = createSignal<Storage[]>([])
  const refresh = async () => {
    const resp = await getStorages()
    handleResp(resp, (data) => setStorages(data.content || []))
  }
  refresh()
  const loadAll = async () => {
    const resp: EmptyResp = await r.post("/admin/storage/load_all")
    handleResp(resp, () => {
      notify.success(t("storages.other.start_load_success"))
      refresh()
    })
  }
  return (
    <VStack spacing="$3" alignItems="start" w="$full">
      <HStack
        spacing="$2"
        gap="$2"
        w="$full"
        wrap={{
          "@initial": "wrap",
          "@md": "unset",
        }}
      >
        <Button
          colorScheme="accent"
          loading={getStoragesLoading()}
          onClick={refresh}
        >
          {t("global.refresh")}
        </Button>
        <Button
          onClick={() => {
            to("/@manage/storages/add")
          }}
        >
          {t("storages.page.add")}
        </Button>
        <Show when={storages().length > 0}>
          <Button
            colorScheme="warning"
            loading={getStoragesLoading()}
            onClick={loadAll}
          >
            {t("storages.other.load_all")}
          </Button>
        </Show>
      </HStack>

      <Show
        when={storages().length > 0}
        fallback={
          <VStack
            w="$full"
            spacing="$3"
            py="$10"
            px="$4"
            alignItems="center"
            border="1px dashed $neutral7"
            rounded="$lg"
          >
            <Text textAlign="center">{t("storages.page.empty")}</Text>
            <Text textAlign="center" color="$neutral10" fontSize="$sm">
              {t("storages.page.empty_hint")}
            </Text>
            <Button
              onClick={() => {
                to("/@manage/storages/add")
              }}
            >
              {t("storages.page.add")}
            </Button>
          </VStack>
        }
      >
        <Box w="$full" overflowX="auto">
          <Table highlightOnHover dense>
            <Thead>
              <Tr>
                <For
                  each={["mount_path", "order", "usage", "status", "remark"]}
                >
                  {(title) => <Th>{t(`storages.common.${title}`)}</Th>}
                </For>
                <Th>{t("global.operations")}</Th>
              </Tr>
            </Thead>
            <Tbody>
              <For each={storages()}>
                {(storage) => (
                  <StorageListItem storage={storage} refresh={refresh} />
                )}
              </For>
            </Tbody>
          </Table>
        </Box>
      </Show>
    </VStack>
  )
}

export default Storages
