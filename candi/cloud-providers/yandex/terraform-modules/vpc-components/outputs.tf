# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

output "route_table_id" {
  value = yandex_vpc_route_table.kube.id
}

output "zone_to_subnet_id_map" {
    value = {
      "ru-central1-a": "e9baudtqor3frm6m5bjg"
      "ru-central1-b": "e2lu8r1tgjmonhdpa9ro"
      "ru-central1-c": "b0ci23brff18p4rnp44a"
    }
}
