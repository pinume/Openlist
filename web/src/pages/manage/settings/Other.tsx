import { Button, Heading, HStack, Input, VStack } from "@hope-ui/solid"
import { createSignal } from "solid-js"
import { MaybeLoading } from "~/components"
import { useFetch, useManageTitle, useT, useUtil } from "~/hooks"
import { Group, PResp, SettingItem } from "~/types"
import { handleResp, notify, r } from "~/utils"

const OtherSettings = () => {
  const t = useT()
  const { copy } = useUtil()
  const [token, setToken] = createSignal("")
  useManageTitle("manage.sidemenu.other")

  const [settingsLoading, settingsData] = useFetch(
    (): PResp<SettingItem[]> =>
      r.get(`/admin/setting/list?group=${Group.SINGLE}`),
  )
  const refresh = async () => {
    const resp = await settingsData()
    handleResp(resp, (data) => {
      setToken(data.find((item) => item.key === "token")?.value || "")
    })
  }
  refresh()

  const [resetTokenLoading, resetToken] = useFetch(
    (): PResp<string> => r.post("/admin/setting/reset_token"),
  )

  return (
    <MaybeLoading loading={settingsLoading()}>
      <VStack alignItems="start" spacing="$2">
        <Heading>{t("settings.token")}</Heading>
        <Input value={token()} readOnly />
        <HStack spacing="$2">
          <Button onClick={() => copy(token())}>
            {t("settings_other.copy_token")}
          </Button>
          <Button
            colorScheme="danger"
            loading={resetTokenLoading()}
            onClick={async () => {
              const resp = await resetToken()
              handleResp(resp, (data) => {
                notify.success(t("settings_other.reset_token_success"))
                setToken(data)
              })
            }}
          >
            {t("settings_other.reset_token")}
          </Button>
        </HStack>
      </VStack>
    </MaybeLoading>
  )
}

export default OtherSettings
