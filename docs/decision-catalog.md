# Decision catalog

The OpenFGA Check each action requires before it is allowed. Every action is allowed only after its check returns true.


Object ids below are bare. The fga infrastructure formats them.

| Action | Check user | Relation | Object |
| --- | --- | --- | --- |
| List all tenants, create tenant, create user, patch user | actor | `can_manage_platform` | `platform:main` |
| Set product | actor | `can_manage_platform` | `platform:main` (system admin only; tenant admins cannot toggle) |
| List my tenants | actor | ListObjects `can_read_tenant` | type `tenant` |
| Get one tenant | actor | `can_manage_platform` on `platform:main` OR `can_read_tenant` on that tenant | either check true |
| List products | actor | `can_manage_platform` on `platform:main` OR `can_view_products` on that tenant | either check true; 403 otherwise |
| List members | actor | `can_manage_platform` on `platform:main` OR `can_manage_members` on that tenant | either check true; 403 otherwise |
| List share candidates for an object | actor | `can_share` | that object |
| Assign, change, or remove a member | actor | `can_manage_platform` on `platform:main` OR `can_manage_members` on that tenant | either check true |
| Create folder | actor | `can_create_folder`; plus `can_create_private_folder` when `is_private`; plus `can_edit` on the parent when `parent_id` is set | that tenant / parent folder |
| Create album | actor | `can_create_album`; plus `can_create_private_album` when `is_private`; plus `can_edit` on the parent | that tenant / parent album |
| Make a folder or album public or private | actor | `can_change_visibility` | that folder or album |
| List folders | actor | ListObjects `can_view` | type `folder` |
| List documents | actor | ListObjects `can_view` | type `document` |
| List albums | actor | ListObjects `can_view` | type `album` |
| List photos | actor | ListObjects `can_view` | type `photo` |
| Get or rename folder, create document in it | actor | `can_view` for get, `can_manage` for rename, `can_create_document` for create | that folder |
| Get document | actor | `can_view` | that document |
| Rename document | actor | `can_edit` | that document |
| Delete document | actor | `can_delete` | that document |
| Share or revoke on a folder or document | actor | `can_share` | that object |
| Album and photo verbs | actor | the album or photo relation with the same name as the folder or document verb | that album or photo |
| Delete folder or album | actor | `can_delete` | that folder or album |

OR in the table means the service calls Check twice (`Authorizer.PlatformOr`) and allows the action when either call is true. That composition stays in the service layer. It is not a role test, and it is not a new OpenFGA relation that would make a system admin a tenant member.

List queries:

```sql
SELECT ... FROM documents
WHERE tenant_id = $tenant AND id = ANY($ids)
```

`$ids` is the ListObjects result. When `$ids` is empty, return an empty JSON array and do not query the table. After the query, drop nothing further except ids ListObjects returned that are not in this tenant. Never `SELECT` every row in the tenant and filter in Go.

The same shape applies to folders, albums, and photos.

Parent folder for `POST /documents`: the folder row's `tenant_id` must equal `X-Tenant-ID`. Otherwise 404.

Depth: reject folder and album creation at 409 when the parent chain is longer than 10. Walk `parent_id` in Postgres and reject a cycle with 409.

