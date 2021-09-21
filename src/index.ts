import {
  join,
} from 'path'
import {
  Construct,
} from '@aws-cdk/core'
import {
  Secret,
} from '@aws-cdk/aws-secretsmanager'
import {
  Repository,
  RepositoryProps,
} from '@aws-cdk/aws-codecommit'
import {
  GoFunction,
} from '@aws-cdk/aws-lambda-go'
import {
  AfterCreate,
} from 'cdk-triggers'

// !ToDo: Use projen (https://www.npmjs.com/package/projen).
// ToDo: Use CDK nag (https://www.npmjs.com/package/cdk-nag).

export interface KeyValue {
  readonly [key: string]: string | number,
}

export interface KeyString {
  readonly [key: string]: string
}

export interface KeyValuePair {
  readonly name?: string,
  readonly value?: string,
}

export interface ForkedRepositoryProps extends RepositoryProps {
  readonly srcRepo: string
  readonly secretName: string
  readonly buildArgs?: KeyString
  readonly goBuildFlags?: string[]
}

export class ForkedRepository extends Repository {

  constructor(scope: Construct, id: string, props: ForkedRepositoryProps) {
    super(scope, id, props)
    const secret = Secret.fromSecretNameV2(this, 'Secret', props.secretName)
    const entry = join(__dirname, 'fork')
    const bundling = {
      buildArgs: props.buildArgs,
      goBuildFlags: props.goBuildFlags,
    }
    const handler = new GoFunction(this, 'Handler', {
      entry,
      bundling,
    })
    handler.addEnvironment('SRC_REPO', props.srcRepo)
    handler.addEnvironment('DEST_REPO', this.repositoryCloneUrlHttp)
    handler.addEnvironment('DEST_SECRET', props.secretName)
    secret.grantRead(handler)
    this.grantPullPush(handler)
    const resources = [
      this,
    ]
    new AfterCreate(this, 'Fork', {
      resources,
      handler,
    })
  }

}
