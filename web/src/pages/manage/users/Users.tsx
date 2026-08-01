import {
  Badge,
  Box,
  Button,
  HStack,
  Table,
  Tbody,
  Td,
  Th,
  Thead,
  Tooltip,
  Tr,
  VStack,
} from "@hope-ui/solid"
import { createSignal, For } from "solid-js"
import {
  useFetch,
  useListFetch,
  useManageTitle,
  useRouter,
  useT,
} from "~/hooks"
import { handleResp, notify, r } from "~/utils"
import {
  UserPermissions,
  UserPermissionBits,
  User,
  UserMethods,
  PPageResp,
  PEmptyResp,
} from "~/types"
import { DeletePopover } from "../common/DeletePopover"
import { Wether } from "~/components"

const Role = (props: { role: number }) => {
  const roles = [
    { name: "general", color: "info" },
    null,
    { name: "admin", color: "accent" },
  ]
  const role = roles[props.role] ?? roles[0]!
  return <Badge colorScheme={role.color as any}>{role.name}</Badge>
}

const Permissions = (props: { user: User }) => {
  const t = useT()
  const color = (can: boolean) => `$${can ? "success" : "danger"}9`
  return (
    <HStack spacing="$0_5">
      <For each={UserPermissions}>
        {(item) => (
          <Tooltip label={t(`users.permissions.${item}`)}>
            <Box
              boxSize="$2"
              rounded="$full"
              bg={color(
                UserMethods.can(props.user, UserPermissionBits[item]),
              )}
            ></Box>
          </Tooltip>
        )}
      </For>
    </HStack>
  )
}

const Users = () => {
  const t = useT()
  useManageTitle("manage.sidemenu.users")
  const { to } = useRouter()
  const [getUsersLoading, getUsers] = useFetch(
    (): PPageResp<User> => r.get("/admin/user/list"),
  )
  const [users, setUsers] = createSignal<User[]>([])
  const refresh = async () => {
    const resp = await getUsers()
    handleResp(resp, (data) => setUsers(data.content))
  }
  refresh()

  const [deleting, deleteUser] = useListFetch(
    (id: number): PEmptyResp => r.post(`/admin/user/delete?id=${id}`),
  )
  const [cancel_2faId, cancel_2fa] = useListFetch(
    (id: number): PEmptyResp => r.post(`/admin/user/cancel_2fa?id=${id}`),
  )
  return (
    <VStack spacing="$2" alignItems="start" w="$full">
      <HStack spacing="$2">
        <Button
          colorScheme="accent"
          loading={getUsersLoading()}
          onClick={refresh}
        >
          {t("global.refresh")}
        </Button>
        <Button
          onClick={() => {
            to("/@manage/users/add")
          }}
        >
          {t("global.add")}
        </Button>
      </HStack>
      <Box w="$full" overflowX="auto">
        <Table highlightOnHover dense>
          <Thead>
            <Tr>
              <For
                each={[
                  "username",
                  "base_path",
                  "role",
                  "permission",
                  "available",
                ]}
              >
                {(title) => <Th>{t(`users.${title}`)}</Th>}
              </For>
              <Th>{t("global.operations")}</Th>
            </Tr>
          </Thead>
          <Tbody>
            <For each={users()}>
              {(user) => (
                <Tr>
                  <Td>{user.username}</Td>
                  <Td>{user.base_path}</Td>
                  <Td>
                    <Role role={user.role} />
                  </Td>
                  <Td>
                    <Permissions user={user} />
                  </Td>
                  <Td>
                    <Wether yes={!user.disabled} />
                  </Td>
                  <Td>
                    <HStack spacing="$2">
                      <Button
                        onClick={() => {
                          to(`/@manage/users/edit/${user.id}`)
                        }}
                      >
                        {t("global.edit")}
                      </Button>
                      <DeletePopover
                        name={user.username}
                        loading={deleting() === user.id}
                        onClick={async () => {
                          const resp = await deleteUser(user.id)
                          handleResp(resp, () => {
                            notify.success(t("global.delete_success"))
                            refresh()
                          })
                        }}
                      />
                      <Button
                        colorScheme="accent"
                        loading={cancel_2faId() === user.id}
                        onClick={async () => {
                          const resp = await cancel_2fa(user.id)
                          handleResp(resp, () => {
                            notify.success(t("users.cancel_2fa_success"))
                            refresh()
                          })
                        }}
                      >
                        {t("users.cancel_2fa")}
                      </Button>
                    </HStack>
                  </Td>
                </Tr>
              )}
            </For>
          </Tbody>
        </Table>
      </Box>
    </VStack>
  )
}

export default Users
