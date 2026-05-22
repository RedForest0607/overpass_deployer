locals {
  account = {
    id     = "380723148945"
    region = "ap-northeast-2"
  }

  vpcs = {
    dev = {
      id   = "vpc-067cb0b0af0f4fed4"
      cidr = "10.20.0.0/16"
      name = "dev-vpc-01"
    }
    prod = {
      id   = "vpc-087035e5939e270fb"
      cidr = "10.10.0.0/16"
      name = "prd-vpc-01"
    }
    istockmall_prod = {
      id   = "vpc-04f0895b77627e829"
      cidr = "10.0.0.0/16"
      name = "istockmall-VPC-prd"
    }
  }

  subnets = {
    dev_public_2a = {
      id     = "subnet-04d61f6127e18bcb7"
      vpc    = "dev"
      az     = "ap-northeast-2a"
      cidr   = "10.20.0.0/24"
      name   = "dev-pub-snet-01-a"
      public = true
    }
    dev_public_2c = {
      id     = "subnet-038678c59d6a089c4"
      vpc    = "dev"
      az     = "ap-northeast-2c"
      cidr   = "10.20.1.0/24"
      name   = "dev-pub-snet-02-c"
      public = true
    }
    dev_app_2a = {
      id     = "subnet-09f3f6440a9b2f3ad"
      vpc    = "dev"
      az     = "ap-northeast-2a"
      cidr   = "10.20.10.0/24"
      name   = "dev-app-pri-snet-01-a"
      public = false
    }
    dev_mgmt_2a = {
      id     = "subnet-0e6d15cbb498fabb2"
      vpc    = "dev"
      az     = "ap-northeast-2a"
      cidr   = "10.20.20.0/24"
      name   = "dev-mgmt-pri-snet-01-a"
      public = false
    }
    dev_db_2a = {
      id     = "subnet-09c3f8f7547bc4afc"
      vpc    = "dev"
      az     = "ap-northeast-2a"
      cidr   = "10.20.30.0/24"
      name   = "dev-db-pri-snet-01-a"
      public = false
    }
    prod_public_2a = {
      id     = "subnet-09793eb9673509b5c"
      vpc    = "prod"
      az     = "ap-northeast-2a"
      cidr   = "10.10.0.0/24"
      name   = "prd-pub-snet-01-a"
      public = true
    }
    prod_public_2c = {
      id     = "subnet-0a32fa202872f8529"
      vpc    = "prod"
      az     = "ap-northeast-2c"
      cidr   = "10.10.1.0/24"
      name   = "prd-pub-snet-02-c"
      public = true
    }
    prod_app_2a = {
      id     = "subnet-038b94b58b3b28aa7"
      vpc    = "prod"
      az     = "ap-northeast-2a"
      cidr   = "10.10.30.0/24"
      name   = "prd-app-pri-snet-01-a"
      public = false
    }
    prod_app_2c = {
      id     = "subnet-0410faa00d5059e50"
      vpc    = "prod"
      az     = "ap-northeast-2c"
      cidr   = "10.10.31.0/24"
      name   = "prd-app-pri-snet-02-c"
      public = false
    }
    prod_backoffice_2a = {
      id     = "subnet-07e335a79c9e82393"
      vpc    = "prod"
      az     = "ap-northeast-2a"
      cidr   = "10.10.40.0/24"
      name   = "prd-backoffice-pri-snet-01-a"
      public = false
    }
    prod_backoffice_2c = {
      id     = "subnet-086096f539a291102"
      vpc    = "prod"
      az     = "ap-northeast-2c"
      cidr   = "10.10.41.0/24"
      name   = "prd-backoffice-pri-snet-02-c"
      public = false
    }
    prod_search_2a = {
      id     = "subnet-0fe7a9cdab3f91c30"
      vpc    = "prod"
      az     = "ap-northeast-2a"
      cidr   = "10.10.50.0/24"
      name   = "prd-search-pri-snet-01-a"
      public = false
    }
    prod_search_2c = {
      id     = "subnet-0bfe5d4215297ec57"
      vpc    = "prod"
      az     = "ap-northeast-2c"
      cidr   = "10.10.51.0/24"
      name   = "prd-search-pri-snet-02-c"
      public = false
    }
    prod_cache_2a = {
      id     = "subnet-054902c46507360d2"
      vpc    = "prod"
      az     = "ap-northeast-2a"
      cidr   = "10.10.60.0/24"
      name   = "prd-cache-pri-snet-01-a"
      public = false
    }
    prod_cache_2c = {
      id     = "subnet-059ba6b8fc9ba3ef6"
      vpc    = "prod"
      az     = "ap-northeast-2c"
      cidr   = "10.10.61.0/24"
      name   = "prd-cache-pri-snet-02-c"
      public = false
    }
    stage_app_2a = {
      id     = "subnet-06ad45d4a6a6667ca"
      vpc    = "prod"
      az     = "ap-northeast-2a"
      cidr   = "10.10.128.0/24"
      name   = "stg-app-pri-snet-01-a"
      public = false
    }
    stage_app_2c = {
      id     = "subnet-0eafea1536c9ed75b"
      vpc    = "prod"
      az     = "ap-northeast-2c"
      cidr   = "10.10.129.0/24"
      name   = "stg-app-pri-snet-02-c"
      public = false
    }
  }

  instances = {
    dev_bastion = {
      instance_id    = "i-0e52c7b8f31fb6573"
      deploy_role    = "bastion"
      environment    = "dev"
      name           = "dev-bastion-ec2"
      private_ip     = "10.20.0.81"
      public_ip      = "54.116.159.65"
      subnet         = "dev_public_2a"
      security_group = "dev_bastion"
    }
    devwas = {
      instance_id    = "i-03b1b81af986a7fbe"
      deploy_role    = "devwas"
      environment    = "dev"
      name           = "dev-was-ec2-01-a"
      private_ip     = "10.20.10.41"
      public_ip      = null
      subnet         = "dev_app_2a"
      security_group = "dev_was"
    }
    devapp = {
      instance_id    = "i-059fa832df15c63e7"
      deploy_role    = "devapp"
      environment    = "dev"
      name           = "dev-app-ec2-01-a"
      private_ip     = "10.20.10.202"
      public_ip      = null
      subnet         = "dev_app_2a"
      security_group = "dev_app"
    }
    devbuild = {
      instance_id    = "i-04fa83b2d365d40f9"
      deploy_role    = "devbuild"
      environment    = "dev"
      name           = "dev-build-ec2-01-a"
      private_ip     = "10.20.20.60"
      public_ip      = null
      subnet         = "dev_mgmt_2a"
      security_group = "dev_build"
    }
    hamonica = {
      instance_id    = "i-0096431de86b6b7ff"
      deploy_role    = "hamonica"
      environment    = "dev"
      name           = "dev-harmonica-ec2-01-a"
      private_ip     = "10.20.20.102"
      public_ip      = null
      subnet         = "dev_mgmt_2a"
      security_group = "dev_harmonica"
    }
    hamonica_adm = {
      instance_id    = "i-0923d8f73dd3dd07d"
      deploy_role    = "hamonica-adm"
      environment    = "dev"
      name           = "dev-harmonica-adm-ec2-01-a"
      private_ip     = "10.20.20.46"
      public_ip      = null
      subnet         = "dev_mgmt_2a"
      security_group = "dev_harmonica_adm"
    }
    postgresql = {
      instance_id    = "i-0706a2dc7090f9bb7"
      deploy_role    = "postgresql"
      environment    = "dev"
      name           = "dev-postgresql-ec2-01-a"
      private_ip     = "10.20.30.87"
      public_ip      = null
      subnet         = "dev_db_2a"
      security_group = "dev_postgresql"
    }
    prd_bastion = {
      instance_id    = "i-0b6a4bfd09914f38e"
      deploy_role    = "bastion"
      environment    = "prod"
      name           = "prd-bastion-ec2"
      private_ip     = "10.10.0.187"
      public_ip      = "15.164.12.24"
      subnet         = "prod_public_2a"
      security_group = "prd_bastion"
    }
    cache1 = {
      instance_id    = "i-01173f3c11ea5b68d"
      deploy_role    = "cache1"
      environment    = "prod"
      name           = "prd-cache-ec2-01-a"
      private_ip     = "10.10.60.15"
      public_ip      = null
      subnet         = "prod_cache_2a"
      security_group = "prd_cache"
    }
    cache2 = {
      instance_id    = "i-06523dff8763a5878"
      deploy_role    = "cache2"
      environment    = "prod"
      name           = "prd-cache-ec2-02-c"
      private_ip     = "10.10.61.40"
      public_ip      = null
      subnet         = "prod_cache_2c"
      security_group = "prd_cache"
    }
    app1 = {
      instance_id    = "i-042f13b9e7f522fc5"
      deploy_role    = "app1"
      environment    = "prod"
      name           = "prd-app-ec2-01-a"
      private_ip     = "10.10.30.222"
      public_ip      = null
      subnet         = "prod_app_2a"
      security_group = "prd_app"
    }
    app2 = {
      instance_id    = "i-009c09a170b2efba3"
      deploy_role    = "app2"
      environment    = "prod"
      name           = "prd-app-ec2-02-c"
      private_ip     = "10.10.31.73"
      public_ip      = null
      subnet         = "prod_app_2c"
      security_group = "prd_app"
    }
    bo1 = {
      instance_id    = "i-093205e93c9f483bb"
      deploy_role    = "bo1"
      environment    = "prod"
      name           = "prd-bo-ec2-01-a"
      private_ip     = "10.10.40.229"
      public_ip      = null
      subnet         = "prod_backoffice_2a"
      security_group = "prd_bo"
    }
    bo2 = {
      instance_id    = "i-09477e205d37c6c08"
      deploy_role    = "bo2"
      environment    = "prod"
      name           = "prd-bo-ec2-02-c"
      private_ip     = "10.10.41.204"
      public_ip      = null
      subnet         = "prod_backoffice_2c"
      security_group = "prd_bo"
    }
    search1 = {
      instance_id    = "i-03cc18038125fc179"
      deploy_role    = "search1"
      environment    = "prod"
      name           = "prd-search-ec2-01-a"
      private_ip     = "10.10.50.106"
      public_ip      = null
      subnet         = "prod_search_2a"
      security_group = "prd_search"
    }
    search2 = {
      instance_id    = "i-06c3ffc3d16b18d43"
      deploy_role    = "search2"
      environment    = "prod"
      name           = "prd-search-ec2-02-c"
      private_ip     = "10.10.51.10"
      public_ip      = null
      subnet         = "prod_search_2c"
      security_group = "prd_search"
    }
    stg_fo_api = {
      instance_id    = "i-0e3dafa24b6ca51d9"
      deploy_role    = "stg-fo-api"
      environment    = "stage"
      name           = "stg-fo-api-ec2-01-a"
      private_ip     = "10.10.128.4"
      public_ip      = null
      subnet         = "stage_app_2a"
      security_group = "stg_fo_api"
    }
    stg_fo_mobile = {
      instance_id    = "i-031211f2a609f358d"
      deploy_role    = "stg-fo-mobile"
      environment    = "stage"
      name           = "stg-fo-mobile-ec2-01-a"
      private_ip     = "10.10.128.11"
      public_ip      = null
      subnet         = "stage_app_2a"
      security_group = "stg_fo_mobile"
    }
    stg_fo_pcweb = {
      instance_id    = "i-0f9070c0858d306b2"
      deploy_role    = "stg-fo-pcweb"
      environment    = "stage"
      name           = "stg-fo-pcweb-ec2-01-a"
      private_ip     = "10.10.128.212"
      public_ip      = null
      subnet         = "stage_app_2a"
      security_group = "stg_fo_pcweb"
    }
  }

  security_groups = {
    dev_bastion       = "sg-0e8443e00e6cb7349"
    dev_was           = "sg-0dcb07f199fbfdf9c"
    dev_app           = "sg-0ab77d4b7242c623b"
    dev_build         = "sg-099bd1900ee6c8ef2"
    dev_harmonica     = "sg-040c0a7c5c75596ef"
    dev_harmonica_adm = "sg-04c26e426b94437ad"
    dev_postgresql    = "sg-06309d70e24967486"
    prd_bastion       = "sg-0f28e61429eca5f3a"
    prd_app           = "sg-0f1bc3d8047dfb652"
    prd_bo            = "sg-0914cf57c481a43cd"
    prd_cache         = "sg-09b0cf8cbcd233fc7"
    prd_search        = "sg-0183bd6ae67570865"
    stg_fo_api        = "sg-07170c7efdf95c818"
    stg_fo_mobile     = "sg-035eb008c693f93d2"
    stg_fo_pcweb      = "sg-031c8e422e77ae40a"
  }

  route_tables = {
    dev_public                 = "rtb-0f7822a9ec6bbd988"
    dev_private                = "rtb-05126e58e1ac9e1dc"
    prod_public                = "rtb-06a2f67c835db19e6"
    prod_private               = "rtb-07aafdb53b8daf018"
    istockmall_prod_public     = "rtb-017a4d7f6c5ed9516"
    istockmall_prod_private_2a = "rtb-0d3d227ed6c2c5049"
    istockmall_prod_private_2c = "rtb-0bd98a47ac0474279"
  }

  nat_gateways = {
    dev           = "nat-0ac651bdb55cfa038"
    prod          = "nat-0e7878dc75ff947ff"
    istockmall_2a = "nat-03dafe9a216618747"
    istockmall_2c = "nat-032e9ba2672e8a2a0"
  }

  load_balancers = {
    dev_external          = "dev-external-alb"
    prd_fo                = "prd-fo-external-alb"
    prd_bo                = "prd-bo-external-alb"
    prd_api               = "prd-api-internal-alb"
    prd_payment           = "prd-payment-internal-alb"
    prd_search            = "prd-search-external-alb"
    prd_apm               = "prd-apm-external-alb"
    istockmall_backend    = "istockmall-ALB-Backend-prd"
    istockmall_backoffice = "istockmall-ALB-BackOffice-prd"
    istockmall_rabbit_mq  = "istockmall-NLB-Rabbit-MQ-prd"
  }

  databases = {
    prod_postgres    = "prd-postgres-rds"
    istockmall_mysql = "isockmall-db"
  }

  deploy_placeholder_values = {
    dev = {
      BASTION_HOST          = local.instances.dev_bastion.private_ip
      DEVWAS_HOST           = local.instances.devwas.private_ip
      DEVAPP_HOST           = local.instances.devapp.private_ip
      DEVBUILD_HOST         = local.instances.devbuild.private_ip
      DEV_HAMONICA_HOST     = local.instances.hamonica.private_ip
      DEV_HAMONICA_ADM_HOST = local.instances.hamonica_adm.private_ip
      DEV_POSTGRESQL_HOST   = local.instances.postgresql.private_ip
    }
    stage = {
      STG_BASTION_HOST   = local.instances.prd_bastion.private_ip
      STG_FO_API_HOST    = local.instances.stg_fo_api.private_ip
      STG_FO_MOBILE_HOST = local.instances.stg_fo_mobile.private_ip
      STG_FO_PCWEB_HOST  = local.instances.stg_fo_pcweb.private_ip
    }
    prod = {
      PRD_BASTION_HOST = local.instances.prd_bastion.private_ip
      PRD_CACHE1_HOST  = local.instances.cache1.private_ip
      PRD_CACHE2_HOST  = local.instances.cache2.private_ip
      PRD_APP1_HOST    = local.instances.app1.private_ip
      PRD_APP2_HOST    = local.instances.app2.private_ip
      PRD_BO1_HOST     = local.instances.bo1.private_ip
      PRD_BO2_HOST     = local.instances.bo2.private_ip
      PRD_SEARCH1_HOST = local.instances.search1.private_ip
      PRD_SEARCH2_HOST = local.instances.search2.private_ip
    }
  }
}
