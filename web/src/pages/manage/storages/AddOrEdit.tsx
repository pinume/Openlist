import {
  Button,
  FormControl,
  FormHelperText,
  FormLabel,
  Heading,
  HStack,
  Input,
  Switch as HopeSwitch,
} from "@hope-ui/solid"
import { createMemo, createSignal, For, Show } from "solid-js"
import { MaybeLoading } from "~/components"
import { useFetch, useRouter, useT } from "~/hooks"
import { handleResp, notify, r } from "~/utils"
import {
  Addition,
  DriverConfig,
  DriverItem,
  PResp,
  Storage,
  Type,
} from "~/types"
import { createStore } from "solid-js/store"
import { Item } from "./Item"
import { ResponsiveGrid } from "../common/ResponsiveGrid"

const LOCAL_DRIVER = "Local"
const RECYCLE_OFF = "delete permanently"
const RECYCLE_DEFAULT = ".recycle"

/** Fields shown by default on the local-mount form. */
const PRIMARY_COMMON = new Set(["mount_path", "remark"])
const PRIMARY_ADDITIONAL = new Set(["root_folder_path", "show_hidden"])
/** Handled by a dedicated recycle-bin control. */
const SPECIAL_ADDITIONAL = new Set(["recycle_bin_path"])

interface DriverInfo {
  common: DriverItem[]
  additional: DriverItem[]
  config: DriverConfig
}

function GetDefaultValue(type: Type, value?: string) {
  switch (type) {
    case Type.Bool:
      if (value) {
        return value === "true"
      }
      return false
    case Type.Number:
      if (value) {
        return parseInt(value)
      }
      return 0
    case Type.Float:
      if (value) {
        return parseFloat(value)
      }
      return 0
    default:
      if (value) {
        return value
      }
      return ""
  }
}

function applyDriverDefaults(
  info: DriverInfo,
  setStorage: (key: keyof Storage, value: any) => void,
  setAddition: (key: string, value: any) => void,
) {
  setStorage("driver", LOCAL_DRIVER)
  for (const item of info.common) {
    setStorage(
      item.name as keyof Storage,
      GetDefaultValue(item.type, item.default) as any,
    )
  }
  for (const item of info.additional) {
    setAddition(item.name, GetDefaultValue(item.type, item.default) as any)
  }
}

const AddOrEdit = () => {
  const t = useT()
  const { params, back, to } = useRouter()
  const { id } = params
  const [driverInfo, setDriverInfo] = createSignal<DriverInfo>()
  const [storage, setStorage] = createStore<Storage>({
    driver: LOCAL_DRIVER,
  } as Storage)
  const [addition, setAddition] = createStore<Addition>({})
  const [advancedOpen, setAdvancedOpen] = createSignal(false)

  const [driversLoading, loadDrivers] = useFetch(
    (): PResp<Record<string, DriverInfo>> => r.get("/admin/driver/list"),
    true,
  )
  const [storageLoading, loadStorage] = useFetch(
    (): PResp<Storage> => r.get(`/admin/storage/get?id=${id}`),
    true,
  )
  const [driverLoading, loadDriver] = useFetch(
    (): PResp<DriverInfo> =>
      r.get(`/admin/driver/info?driver=${storage.driver || LOCAL_DRIVER}`),
    true,
  )

  const initAdd = async () => {
    const resp = await loadDrivers()
    handleResp(resp, (data) => {
      const local = data[LOCAL_DRIVER]
      if (!local) {
        notify.error(t("storages.other.local_driver_unavailable"))
        return
      }
      setDriverInfo(local)
      applyDriverDefaults(local, setStorage, setAddition)
    })
  }

  const initEdit = async () => {
    const storageResp = await loadStorage()
    handleResp(storageResp, async (storageData) => {
      setStorage(storageData)
      try {
        setAddition(JSON.parse(storageData.addition || "{}"))
      } catch {
        setAddition({})
      }
      const driverResp = await loadDriver()
      handleResp(driverResp, (driverData) => {
        setDriverInfo(driverData)
      })
    })
  }

  if (id) {
    initEdit()
  } else {
    initAdd()
  }

  const [okLoading, ok] = useFetch((): PResp<{ id: number }> => {
    setStorage("addition", JSON.stringify(addition))
    setStorage("driver", storage.driver || LOCAL_DRIVER)
    return r.post(`/admin/storage/${id ? "update" : "create"}`, storage)
  })

  const info = createMemo(() => driverInfo())

  const itemByName = (list: DriverItem[] | undefined, name: string) =>
    list?.find((i) => i.name === name)

  const advancedCommon = createMemo(
    () => info()?.common.filter((i) => !PRIMARY_COMMON.has(i.name)) ?? [],
  )
  const advancedAdditional = createMemo(
    () =>
      info()?.additional.filter(
        (i) =>
          !PRIMARY_ADDITIONAL.has(i.name) && !SPECIAL_ADDITIONAL.has(i.name),
      ) ?? [],
  )

  // ItemProps is a discriminated union on `type`; DriverItem from the API is not narrowed.
  const renderCommonItem = (item: DriverItem) => (
    <Item
      {...(item as any)}
      driver="common"
      value={(storage as any)[item.name]}
      onChange={(val: any) => {
        setStorage(item.name as keyof Storage, val)
      }}
    />
  )

  const renderAdditionalItem = (item: DriverItem) => (
    <Item
      {...(item as any)}
      driver={storage.driver || LOCAL_DRIVER}
      value={addition[item.name] as any}
      onChange={(val: any) => {
        setAddition(item.name, val)
      }}
    />
  )

  const recycleEnabled = () => {
    const v = String(addition.recycle_bin_path ?? "")
    return v !== "" && v !== RECYCLE_OFF
  }

  const recyclePath = () => {
    if (!recycleEnabled()) return RECYCLE_DEFAULT
    return String(addition.recycle_bin_path || RECYCLE_DEFAULT)
  }

  const setRecycleEnabled = (enabled: boolean) => {
    if (enabled) {
      const cur = String(addition.recycle_bin_path ?? "")
      setAddition(
        "recycle_bin_path",
        cur && cur !== RECYCLE_OFF ? cur : RECYCLE_DEFAULT,
      )
    } else {
      setAddition("recycle_bin_path", RECYCLE_OFF)
    }
  }

  return (
    <MaybeLoading
      loading={id ? storageLoading() || driverLoading() : driversLoading()}
    >
      <Heading mb="$2">
        {t(id ? "storages.page.edit" : "storages.page.add")}
      </Heading>
      <ResponsiveGrid>
        <Show when={info()}>
          {/* Primary order: site path → server dir → recycle → hidden → remark */}
          <Show when={itemByName(info()!.common, "mount_path")}>
            {(item) => renderCommonItem(item())}
          </Show>
          <Show when={itemByName(info()!.additional, "root_folder_path")}>
            {(item) => renderAdditionalItem(item())}
          </Show>
          <FormControl w="$full" display="flex" flexDirection="column">
            <FormLabel display="flex" alignItems="center">
              {t("storages.fields.recycle_enable")}
            </FormLabel>
            <HopeSwitch
              checked={recycleEnabled()}
              onChange={(e: Event) => {
                setRecycleEnabled(
                  (e.currentTarget as HTMLInputElement).checked,
                )
              }}
            />
            <FormHelperText>{t("storages.fields.recycle_tips")}</FormHelperText>
          </FormControl>
          <Show when={recycleEnabled()}>
            <FormControl w="$full" display="flex" flexDirection="column">
              <FormLabel display="flex" alignItems="center">
                {t("storages.fields.recycle_path")}
              </FormLabel>
              <Input
                value={recyclePath()}
                onChange={(e) => {
                  const v = e.currentTarget.value.trim()
                  setAddition(
                    "recycle_bin_path",
                    v === "" || v === RECYCLE_OFF ? RECYCLE_DEFAULT : v,
                  )
                }}
                placeholder={RECYCLE_DEFAULT}
              />
              <FormHelperText>
                {t("storages.fields.recycle_tips")}
              </FormHelperText>
            </FormControl>
          </Show>
          <Show when={itemByName(info()!.additional, "show_hidden")}>
            {(item) => renderAdditionalItem(item())}
          </Show>
          <Show when={itemByName(info()!.common, "remark")}>
            {(item) => renderCommonItem(item())}
          </Show>
        </Show>
      </ResponsiveGrid>

      <Button
        mt="$3"
        variant="ghost"
        colorScheme="neutral"
        onClick={() => setAdvancedOpen(!advancedOpen())}
      >
        {t("storages.other.advanced")}
        {advancedOpen() ? " ▲" : " ▼"}
      </Button>
      <Show when={advancedOpen() && info()}>
        <ResponsiveGrid>
          <For each={advancedCommon()}>
            {(item) => renderCommonItem(item)}
          </For>
          <For each={advancedAdditional()}>
            {(item) => renderAdditionalItem(item)}
          </For>
        </ResponsiveGrid>
      </Show>

      <HStack
        mt="$3"
        spacing="$2"
        gap="$2"
        w="$full"
        wrap={{
          "@initial": "wrap",
          "@md": "unset",
        }}
      >
        <Button
          loading={okLoading()}
          onClick={async () => {
            const resp = await ok()
            handleResp(
              resp,
              () => {
                notify.success(t("storages.other.save_success_hint"))
                back()
              },
              (_msg, _code) => {
                if (resp.data?.id) {
                  to(`/@manage/storages/edit/${resp.data.id}`)
                }
              },
            )
          }}
        >
          {t(`global.${id ? "save" : "add"}`)}
        </Button>
        <Button colorScheme="neutral" variant="ghost" onClick={() => back()}>
          {t("global.back")}
        </Button>
      </HStack>
    </MaybeLoading>
  )
}

export default AddOrEdit
